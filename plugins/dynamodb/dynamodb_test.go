package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register DynamoDB plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("DynamoDB must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialCloudAccessKey, protocolName) {
		t.Fatal("cloud access key credential should support DynamoDB")
	}
	for _, kind := range []plugin.CredentialKind{plugin.CredentialDBPassword, plugin.CredentialTLSClientCert, plugin.CredentialBasicAuth, plugin.CredentialBearerToken} {
		if reg.CredentialKindSupportsProtocol(kind, protocolName) {
			t.Fatalf("DynamoDB should not advertise %s credentials", kind)
		}
	}
}

func TestConfigSchemaHasOnlyDynamoDBFields(t *testing.T) {
	fields := fieldMap(configSchema())
	for _, key := range []string{"region", "endpoint", "table_prefix", "auth", "access_key_id", "secret_access_key", "session_token", "credential_id", "tls_mode", "ca_certificate", "read_only", "confirm_writes", "timeout", "page_limit"} {
		if !fields[key] {
			t.Fatalf("schema should expose %q", key)
		}
	}
	for _, key := range []string{"username", "password", "database", "host", "port", "api_key", "bearer_token", "query_language", "client_cert_id", "auth_client_cert_id"} {
		if fields[key] {
			t.Fatalf("schema should not expose unrelated field %q", key)
		}
	}
}

func TestConfigSchemaVisibleValuesAreAuthSpecific(t *testing.T) {
	schema := configSchema()
	tests := []struct {
		name   string
		values map[string]any
		want   []string
		reject []string
	}{
		{
			name:   "access key",
			values: map[string]any{"region": "us-east-1", "auth": "access_key", "access_key_id": "akid", "secret_access_key": "secret", "session_token": ""},
			want:   []string{"region", "auth", "access_key_id", "secret_access_key", "session_token", "tls_mode", "read_only", "confirm_writes", "timeout", "page_limit"},
			reject: []string{"credential_id"},
		},
		{
			name:   "stored credential",
			values: map[string]any{"region": "us-east-1", "auth": "credential", "credential_id": "cred-1", "session_token": ""},
			want:   []string{"region", "auth", "credential_id", "session_token", "tls_mode", "read_only", "confirm_writes", "timeout", "page_limit"},
			reject: []string{"access_key_id", "secret_access_key"},
		},
		{
			name:   "provider chain",
			values: map[string]any{"region": "us-east-1", "auth": "default_chain"},
			want:   []string{"region", "auth", "tls_mode", "read_only", "confirm_writes", "timeout", "page_limit"},
			reject: []string{"access_key_id", "secret_access_key", "session_token", "credential_id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible := schema.VisibleValues(schema.ValuesWithDefaults(tt.values), nil)
			for _, key := range tt.want {
				if _, ok := visible[key]; !ok {
					t.Fatalf("visible values should include %q in %#v", key, visible)
				}
			}
			for _, key := range tt.reject {
				if _, ok := visible[key]; ok {
					t.Fatalf("visible values should not include %q in %#v", key, visible)
				}
			}
		})
	}
}

func TestPartiQLSafetyStopsBeforeNetwork(t *testing.T) {
	_, err := executePartiQL(context.Background(), &Session{opts: Options{ReadOnly: true}}, sqldb.QueryRequest{Query: `DELETE FROM "users" WHERE pk='1'`})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executePartiQL(context.Background(), &Session{opts: Options{ConfirmWrites: true}}, sqldb.QueryRequest{Query: `INSERT INTO "users" VALUE {'pk':'1'}`})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestAttributeValueKeyRoundTrip(t *testing.T) {
	key := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "user#1"},
		"sk": &types.AttributeValueMemberN{Value: "42"},
	}
	id, err := encodeItemID("users", key)
	if err != nil {
		t.Fatalf("encode item id: %v", err)
	}
	table, decoded, err := decodeItemID(id)
	if err != nil {
		t.Fatalf("decode item id: %v", err)
	}
	if table != "users" || keyDisplay(decoded, []types.KeySchemaElement{{AttributeName: strptr("pk"), KeyType: types.KeyTypeHash}, {AttributeName: strptr("sk"), KeyType: types.KeyTypeRange}}) != "pk=user#1 · sk=42" {
		t.Fatalf("unexpected decoded key: table=%s key=%#v", table, decoded)
	}
}

func TestManifestReferencesResolve(t *testing.T) {
	p := New()
	m := p.Manifest()
	routeIDs := map[string]bool{}
	for _, r := range p.Routes() {
		routeIDs[r.ID] = true
	}
	actionByID := map[string]plugin.Action{}
	for _, action := range m.Actions {
		actionByID[action.ID] = action
		if !routeIDs[action.RouteID] {
			t.Fatalf("action %q points at missing route %q", action.ID, action.RouteID)
		}
	}
	for _, group := range m.Tree {
		if !routeIDs[group.Source.RouteID] {
			t.Fatalf("tree group %q points at missing route %q", group.Key, group.Source.RouteID)
		}
	}
	for _, res := range m.Resources {
		if !routeIDs[res.List.RouteID] {
			t.Fatalf("resource %q list points at missing route %q", res.Kind, res.List.RouteID)
		}
		for _, id := range append(append([]string{}, res.ActionIDs...), append(res.ListActionIDs, res.RowActionIDs...)...) {
			if _, ok := actionByID[id]; !ok {
				t.Fatalf("resource %q references missing action %q", res.Kind, id)
			}
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Source != nil && !routeIDs[tab.Source.RouteID] {
				t.Fatalf("resource %q tab %q points at missing route %q", res.Kind, tab.Key, tab.Source.RouteID)
			}
		}
	}
}

func fieldMap(schema plugin.Schema) map[string]bool {
	out := map[string]bool{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			out[field.Key] = true
		}
	}
	return out
}

func strptr(s string) *string { return &s }
