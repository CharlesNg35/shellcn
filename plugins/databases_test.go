package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestDatabaseCredentialSelectorsExposeOnlyAppropriateKinds(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"postgresql", "mysql", "redis", "mongodb"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		field, ok := credentialField(m.Config, "credential_id")
		if !ok {
			t.Fatalf("%s should expose credential_id", name)
		}
		if !credentialKindsContain(field.Credential.Kinds, plugin.CredentialDBPassword) {
			t.Fatalf("%s auth credential selector should support database password: %+v", name, field.Credential.Kinds)
		}
		if credentialKindsContain(field.Credential.Kinds, plugin.CredentialTLSClientCert) {
			t.Fatalf("%s stored password selector should not advertise TLS client certificates: %+v", name, field.Credential.Kinds)
		}
	}

	for _, name := range []string{"postgresql", "mongodb"} {
		m, _ := reg.Manifest(name)
		field, ok := credentialField(m.Config, "auth_client_cert_id")
		if !ok {
			t.Fatalf("%s should expose auth_client_cert_id for certificate authentication", name)
		}
		if !credentialKindsContain(field.Credential.Kinds, plugin.CredentialTLSClientCert) || credentialKindsContain(field.Credential.Kinds, plugin.CredentialDBPassword) {
			t.Fatalf("%s certificate auth selector should only advertise TLS client certificates: %+v", name, field.Credential.Kinds)
		}
	}

	for _, name := range []string{"mysql", "redis"} {
		m, _ := reg.Manifest(name)
		if _, ok := credentialField(m.Config, "auth_client_cert_id"); ok {
			t.Fatalf("%s should not expose certificate authentication", name)
		}
	}

	for _, name := range []string{"postgresql", "mysql", "redis", "mongodb"} {
		m, _ := reg.Manifest(name)
		tlsField, ok := credentialField(m.Config, "client_cert_id")
		if !ok {
			t.Fatalf("%s should expose client_cert_id in TLS settings", name)
		}
		if !credentialKindsContain(tlsField.Credential.Kinds, plugin.CredentialTLSClientCert) {
			t.Fatalf("%s TLS client certificate field should support TLS client certificates: %+v", name, tlsField.Credential.Kinds)
		}
	}
}

func TestDatabaseConfigVisibleValuesAreAuthSpecific(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"postgresql", "mysql", "mongodb"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		visible := visibleDatabaseFields(m.Config, map[string]any{"auth": "password", "tls_mode": "disable", "encrypt": "disable"})
		requireVisible(t, name, visible, "username", "password")
		requireHidden(t, name, visible, "credential_id", "auth_client_cert_id")
	}

	for _, name := range []string{"postgresql", "mysql", "redis", "mongodb"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		visible := visibleDatabaseFields(m.Config, map[string]any{"auth": "credential", "tls_mode": "disable", "encrypt": "disable"})
		requireVisible(t, name, visible, "credential_id")
		requireHidden(t, name, visible, "username", "password", "auth_client_cert_id")
	}

	for _, name := range []string{"redis"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		visible := visibleDatabaseFields(m.Config, map[string]any{"auth": "none", "tls_mode": "disable"})
		requireHidden(t, name, visible, "username", "password", "credential_id", "auth_client_cert_id")
	}

	for _, name := range []string{"postgresql", "mongodb"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		visible := visibleDatabaseFields(m.Config, map[string]any{"auth": "client_certificate", "tls_mode": "disable"})
		requireVisible(t, name, visible, "auth_client_cert_id")
		requireHidden(t, name, visible, "password", "credential_id", "client_cert_id")
	}
}

func TestDatabaseCreateActionsAreDeclaredAtCollectionLevel(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, tc := range []struct {
		protocol string
		kind     string
		actionID string
		routeID  string
	}{
		{"postgresql", "database", "postgresql.database.create", "postgresql.database.create"},
		{"mysql", "database", "mysql.database.create", "mysql.database.create"},
	} {
		m, ok := reg.Manifest(tc.protocol)
		if !ok {
			t.Fatalf("plugin %q was not registered", tc.protocol)
		}
		res, ok := resourceByKind(m, tc.kind)
		if !ok {
			t.Fatalf("%s should expose resource %q", tc.protocol, tc.kind)
		}
		if !stringSliceContains(res.Actions.Toolbar, tc.actionID) {
			t.Fatalf("%s %s list actions = %#v, want %s", tc.protocol, tc.kind, res.Actions.Toolbar, tc.actionID)
		}
		if !manifestHasAction(m, tc.actionID, tc.routeID) {
			t.Fatalf("%s action %s should route to %s", tc.protocol, tc.actionID, tc.routeID)
		}
		if _, ok := reg.Route(tc.protocol, tc.routeID); !ok {
			t.Fatalf("%s route %s was not registered", tc.protocol, tc.routeID)
		}
	}

	m, ok := reg.Manifest("mongodb")
	if !ok {
		t.Fatal("mongodb was not registered")
	}
	database, ok := resourceByKind(m, "database")
	if !ok {
		t.Fatal("mongodb should expose database resources")
	}
	var collections plugin.Panel
	for _, tab := range database.Detail.Tabs {
		if tab.Key == "collections" {
			collections = tab
			break
		}
	}
	collCfg, _ := collections.Config.(plugin.TableConfig)
	if !stringSliceContains(collCfg.ActionIDs, "mongodb.collection.create") {
		t.Fatalf("mongodb collections tab actions = %#v, want collection create", collections.Config)
	}
	if !manifestHasAction(m, "mongodb.collection.create", "mongodb.collection.create") {
		t.Fatal("mongodb collection create action should route to mongodb.collection.create")
	}
	if _, ok := reg.Route("mongodb", "mongodb.collection.create"); !ok {
		t.Fatal("mongodb collection create route was not registered")
	}
}

func TestEditableDatabaseTablesDeclareColumnMetadataSource(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, tc := range []struct {
		protocol string
		routeID  string
	}{
		{"postgresql", "postgresql.table.columns"},
		{"mysql", "mysql.table.columns"},
	} {
		m, ok := reg.Manifest(tc.protocol)
		if !ok {
			t.Fatalf("plugin %q was not registered", tc.protocol)
		}
		res, ok := resourceByKind(m, "table")
		if !ok {
			t.Fatalf("%s should expose table resources", tc.protocol)
		}
		var data plugin.Panel
		for _, tab := range res.Detail.Tabs {
			if tab.Key == "data" {
				data = tab
				break
			}
		}
		tbl, ok := data.Config.(plugin.TableConfig)
		if !ok || tbl.ColumnsSource == nil {
			t.Fatalf("%s data grid should declare columnsSource: %#v", tc.protocol, data.Config)
		}
		source := tbl.ColumnsSource
		if source.RouteID != tc.routeID {
			t.Fatalf("%s columnsSource = %q, want %q", tc.protocol, source.RouteID, tc.routeID)
		}
		if _, ok := reg.Route(tc.protocol, tc.routeID); !ok {
			t.Fatalf("%s route %s was not registered", tc.protocol, tc.routeID)
		}
	}
}

func credentialField(schema plugin.Schema, key string) (plugin.Field, bool) {
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key && field.Type == plugin.FieldCredentialRef && field.Credential != nil {
				return field, true
			}
		}
	}
	return plugin.Field{}, false
}

func resourceByKind(m plugin.Manifest, kind string) (plugin.ResourceType, bool) {
	for _, res := range m.Resources {
		if res.Kind == kind {
			return res, true
		}
	}
	return plugin.ResourceType{}, false
}

func manifestHasAction(m plugin.Manifest, actionID string, routeID string) bool {
	for _, action := range m.Actions {
		if action.ID == actionID && action.RouteID == routeID {
			return true
		}
	}
	return false
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func credentialKindsContain(kinds []plugin.CredentialKind, want plugin.CredentialKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}

func visibleDatabaseFields(schema plugin.Schema, overrides map[string]any) map[string]bool {
	values := schema.Defaults()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if _, ok := values[field.Key]; !ok {
				values[field.Key] = blankDatabaseFieldValue(field.Type)
			}
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	visible := schema.VisibleValues(values, nil)
	out := map[string]bool{}
	for key := range visible {
		out[key] = true
	}
	return out
}

func blankDatabaseFieldValue(t plugin.FieldType) any {
	switch t {
	case plugin.FieldToggle:
		return false
	case plugin.FieldMultiSelect:
		return []any{}
	default:
		return ""
	}
}

func requireVisible(t *testing.T, pluginName string, visible map[string]bool, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if !visible[key] {
			t.Fatalf("%s should show %q for this auth mode; visible=%v", pluginName, key, visible)
		}
	}
}

func requireHidden(t *testing.T, pluginName string, visible map[string]bool, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if visible[key] {
			t.Fatalf("%s should hide %q for this auth mode; visible=%v", pluginName, key, visible)
		}
	}
}
