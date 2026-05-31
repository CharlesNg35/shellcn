package postgresql

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register PostgreSQL plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("PostgreSQL must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support PostgreSQL")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("TLS client certificate credential should support PostgreSQL")
	}
}

func TestQuerySafetyStopsBeforeDatabase(t *testing.T) {
	// Safety gates return before the pool is touched, so a nil pool is fine here.
	_, err := executeQueryRequest(context.Background(), &Session{opts: options{ReadOnly: true}}, nil, sqldb.QueryRequest{Query: "delete from accounts"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeQueryRequest(context.Background(), &Session{opts: options{RequireConfirm: true}}, nil, sqldb.QueryRequest{Query: "drop table accounts"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if got := queryAuditResult(err); got != models.AuditDenied {
		t.Fatalf("confirmation should audit as denied, got %s", got)
	}
}

func TestParseOptionsUsesTLSCredentialAsClientCertificate(t *testing.T) {
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
		"host":                          "db.local",
		"database":                      "postgres",
		"auth":                          authClientCert,
		"_auth_client_cert_id_identity": "cert-user",
		"_auth_client_cert_id_secret":   "pem-material",
	}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.Username != "cert-user" || opts.Password != "" || opts.ClientCertificate != "pem-material" || opts.TLSMode != "require" {
		t.Fatalf("unexpected credential material: %+v", opts)
	}
}

func TestRedactRowsMasksConfiguredColumnsButKeepsRowKey(t *testing.T) {
	rows := []row{{"id": int64(1), "password": "plain", "name": "alice", "_key": map[string]any{"id": int64(1)}}}
	redactRows(rows, sqldb.DefaultRedactColumnPatterns())
	if rows[0]["password"] != sqldb.RedactedValue || rows[0]["name"] != "alice" {
		t.Fatalf("unexpected row redaction: %#v", rows)
	}
	if _, ok := rows[0]["_key"].(map[string]any); !ok {
		t.Fatalf("_key must survive redaction so edits stay possible: %#v", rows[0])
	}
}

func TestAttachRowKeysOnlyWithPrimaryKey(t *testing.T) {
	withPK := []row{{"id": int64(7), "name": "a"}}
	attachRowKeys(withPK, []string{"id"}, nil)
	key, ok := withPK[0]["_key"].(map[string]any)
	if !ok || key["id"] != int64(7) {
		t.Fatalf("expected _key from primary key, got %#v", withPK[0])
	}
	keyless := []row{{"name": "a"}}
	attachRowKeys(keyless, nil, nil)
	if _, ok := keyless[0]["_key"]; ok {
		t.Fatal("keyless tables must stay read-only (no _key)")
	}
	// A primary key that is itself sensitive must not be shipped raw via _key.
	secretPK := []row{{"api_key": "live_xyz", "name": "a"}}
	attachRowKeys(secretPK, []string{"api_key"}, sqldb.DefaultRedactColumnPatterns())
	if _, ok := secretPK[0]["_key"]; ok {
		t.Fatal("tables keyed by a sensitive column must stay read-only (no _key leak)")
	}
}

// TestManifestReferencesResolve guards the declarative wiring: every route ID a
// tab, data grid, or action points at must actually exist, or the UI breaks.
func TestManifestReferencesResolve(t *testing.T) {
	p := New()
	m := p.Manifest()
	routeIDs := map[string]bool{}
	for _, r := range p.Routes() {
		routeIDs[r.ID] = true
	}
	actionByID := map[string]plugin.Action{}
	for _, a := range m.Actions {
		actionByID[a.ID] = a
		if !routeIDs[a.RouteID] {
			t.Fatalf("action %q points at missing route %q", a.ID, a.RouteID)
		}
	}
	checkAction := func(id string) {
		if _, ok := actionByID[id]; !ok {
			t.Fatalf("referenced action %q is not declared", id)
		}
	}
	checkSource := func(key string, ds *plugin.DataSource) {
		if ds != nil && !routeIDs[ds.RouteID] {
			t.Fatalf("config %q points at missing route %q", key, ds.RouteID)
		}
	}
	for _, g := range m.Tree {
		if !routeIDs[g.Source.RouteID] {
			t.Fatalf("tree group %q points at missing route %q", g.Key, g.Source.RouteID)
		}
	}
	for _, res := range m.Resources {
		if !routeIDs[res.List.RouteID] {
			t.Fatalf("resource %q list points at missing route %q", res.Kind, res.List.RouteID)
		}
		for _, id := range append(append([]string{}, res.ListActionIDs...), res.RowActionIDs...) {
			checkAction(id)
		}
		for _, id := range res.Detail.Header.ActionIDs {
			checkAction(id)
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Source != nil && !routeIDs[tab.Source.RouteID] {
				t.Fatalf("resource %q tab %q points at missing route %q", res.Kind, tab.Key, tab.Source.RouteID)
			}
			if tc, ok := tab.Config.(plugin.TableConfig); ok {
				checkSource("insert", tc.Insert)
				checkSource("update", tc.Update)
				checkSource("delete", tc.Delete)
			}
		}
	}
}

// TestTableDataGridIsEditable locks in the headline professional feature: the
// table Data tab is an editable grid wired to row insert/update/delete.
func TestTableDataGridIsEditable(t *testing.T) {
	m := New().Manifest()
	var data plugin.Tab
	for _, res := range m.Resources {
		if res.Kind != "table" {
			continue
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Key == "data" {
				data = tab
			}
		}
	}
	if data.Key == "" {
		t.Fatal("table resource is missing a Data tab")
	}
	tc, ok := data.Config.(plugin.TableConfig)
	if !ok || !tc.Editable {
		t.Fatalf("Data tab must be editable: %#v", data.Config)
	}
	for key, ds := range map[string]*plugin.DataSource{"insert": tc.Insert, "update": tc.Update, "delete": tc.Delete} {
		if ds == nil {
			t.Fatalf("Data tab missing %q mutation source: %#v", key, data.Config)
		}
	}
}

func TestTreeIsDatabaseRooted(t *testing.T) {
	m := New().Manifest()
	if len(m.Tree) != 1 || m.Tree[0].Key != "databases" {
		t.Fatalf("tree should root at a single Databases group, got %#v", m.Tree)
	}
}
