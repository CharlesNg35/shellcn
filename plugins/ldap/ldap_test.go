package ldap

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	p := New()
	m := p.Manifest()
	plugintest.ValidatePlugin(t, p)
	if m.Agent != nil {
		t.Fatal("LDAP must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if m.Category != plugin.CategorySecurity {
		t.Fatalf("unexpected category: %q", m.Category)
	}
	if got := m.Config.Defaults()["read_only"]; got != true {
		t.Fatalf("read_only manifest default = %#v, want true", got)
	}
}

func TestAttributesTabIsStagedEditableGrid(t *testing.T) {
	tab := attributesTab(t)
	tc, ok := tab.Config.(plugin.TableConfig)
	if !ok || !tc.Editable || !tc.StagedEdits {
		t.Fatalf("attributes grid must be an editable staged grid: %#v", tab.Config)
	}
	if len(tc.RowKey) != 1 || tc.RowKey[0] != "attribute" {
		t.Fatalf("attributes grid rowKey = %#v, want [attribute]", tc.RowKey)
	}
}

func TestEntryActionsAreGatedAndConfirmRiskyOperations(t *testing.T) {
	actions := map[string]plugin.Action{}
	for _, action := range New().Manifest().Actions {
		actions[action.ID] = action
	}
	for _, id := range []string{"ldap.entry.add", "ldap.entry.rename", "ldap.entry.delete"} {
		action, ok := actions[id]
		if !ok {
			t.Fatalf("missing action %s", id)
		}
		if action.EnabledWhen == nil {
			t.Fatalf("%s should be disabled when the connection is read-only", id)
		}
	}
	if !actions["ldap.entry.rename"].Confirm {
		t.Fatal("rename/move should require confirmation")
	}
	if actions["ldap.entry.delete"].OnSuccess == nil || actions["ldap.entry.delete"].OnSuccess.Navigate != plugin.NavigateList {
		t.Fatal("delete should navigate back to the list after success")
	}
}

func attributesTab(t *testing.T) plugin.Panel {
	t.Helper()
	for _, res := range New().Manifest().Resources {
		if res.Kind != "entry" {
			continue
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Key == "attributes" {
				return tab
			}
		}
	}
	t.Fatal("entry resource has no attributes tab")
	return plugin.Panel{}
}

func TestAuthDefaultsToAnonymous(t *testing.T) {
	m := New().Manifest()
	visible := m.Config.VisibleValues(m.Config.ValuesWithDefaults(map[string]any{}), nil)
	if visible["auth"] != authAnonymous {
		t.Fatalf("default auth = %#v, want anonymous", visible["auth"])
	}
	if _, ok := visible["bind_dn"]; ok {
		t.Fatal("bind_dn should be hidden when auth is anonymous")
	}
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{"host": "127.0.0.1"}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.BindDN != "" || opts.Password != "" {
		t.Fatalf("anonymous auth should not set credentials: %+v", opts)
	}
	if !opts.ReadOnly {
		t.Fatal("read-only mode should be enabled by default")
	}
}

func TestParseOptionsSimpleBindRequiresDN(t *testing.T) {
	if _, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{"host": "h", "auth": authSimple, "password": "x"}}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("simple bind without DN should fail, got %v", err)
	}
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
		"host": "h", "auth": authSimple, "bind_dn": "cn=admin,dc=example,dc=com", "password": "secret", "read_only": false,
	}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.BindDN != "cn=admin,dc=example,dc=com" || opts.Password != "secret" {
		t.Fatalf("unexpected bind material: %+v", opts)
	}
	if opts.ReadOnly {
		t.Fatal("read_only should be disabled when configured")
	}
}

func TestParseOptionsRequiresHost(t *testing.T) {
	if _, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{}}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("missing host should fail, got %v", err)
	}
}

func TestSearchFilter(t *testing.T) {
	got, err := searchFilter("")
	if err != nil || got != "(objectClass=*)" {
		t.Fatalf("empty filter = %q", got)
	}
	got, err = searchFilter("(uid=jdoe)")
	if err != nil || got != "(uid=jdoe)" {
		t.Fatalf("raw filter should pass through, got %q", got)
	}
	got, err = searchFilter("ada")
	if err != nil {
		t.Fatalf("free-text filter returned error: %v", err)
	}
	if !strings.Contains(got, "(cn=*ada*)") || !strings.HasPrefix(got, "(|") {
		t.Fatalf("free-text filter = %q", got)
	}
	// Injection metacharacters must be escaped, not passed raw.
	got, err = searchFilter("a)(uid=*")
	if err != nil {
		t.Fatalf("escaped free-text returned error: %v", err)
	}
	if strings.Contains(got, "a)(uid=*") {
		t.Fatalf("filter value was not escaped: %q", got)
	}
	if _, err := searchFilter("(uid=jdoe"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("invalid raw filter should fail as invalid input, got %v", err)
	}
}

func TestAttributeValueRoundTrip(t *testing.T) {
	if got := attributeValue([]string{"only"}); got != "only" {
		t.Fatalf("single value = %#v, want \"only\"", got)
	}
	if got := attributeValue([]string{"a", "b"}); got != `["a","b"]` {
		t.Fatalf("multi value = %#v, want JSON array", got)
	}
	if got := attributeValue(nil); got != "" {
		t.Fatalf("empty value = %#v, want empty string", got)
	}

	if got := attributeValues("single"); !reflect.DeepEqual(got, []string{"single"}) {
		t.Fatalf("scalar parse = %#v", got)
	}
	if got := attributeValues(`["a","b"]`); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("json array parse = %#v", got)
	}
	if got := attributeValues([]any{"x", 1}); !reflect.DeepEqual(got, []string{"x", "1"}) {
		t.Fatalf("list parse = %#v", got)
	}
	if got := attributeValues("  "); got != nil {
		t.Fatalf("blank parse = %#v, want nil", got)
	}
}

func TestRDNAndParent(t *testing.T) {
	dn := "uid=jdoe,ou=people,dc=example,dc=com"
	if got := rdnOf(dn); got != "uid=jdoe" {
		t.Fatalf("rdnOf = %q", got)
	}
	if got := parentOf(dn); got != "ou=people,dc=example,dc=com" {
		t.Fatalf("parentOf = %q", got)
	}
	if got := parentOf("dc=com"); got != "" {
		t.Fatalf("parentOf root = %q, want empty", got)
	}
	escaped := `cn=Doe\, Jane,ou=people,dc=example,dc=com`
	if got := rdnOf(escaped); got != `cn=Doe\, Jane` {
		t.Fatalf("rdnOf escaped = %q", got)
	}
	if got := parentOf(escaped); got != "ou=people,dc=example,dc=com" {
		t.Fatalf("parentOf escaped = %q", got)
	}
	if err := validateRDN("cn=Jane,Doe", "RDN"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("multi-part RDN should fail, got %v", err)
	}
}

func TestEnsureWritableBlocksReadOnly(t *testing.T) {
	if err := ensureWritable(&Session{opts: options{ReadOnly: true}}); !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("read-only should forbid writes, got %v", err)
	}
	if err := ensureWritable(&Session{opts: options{ReadOnly: false}}); err != nil {
		t.Fatalf("writable session should allow writes, got %v", err)
	}
}

func TestIconForEntry(t *testing.T) {
	cases := map[string]string{
		"organizationalUnit":       "folder",
		"inetOrgPerson":            "user",
		"groupOfNames":             "users",
		"widget":                   "file",
		"computer":                 "monitor",
		"group":                    "users",
		"user":                     "user",
		"domainDNS":                "folder",
		"foreignSecurityPrincipal": "user",
	}
	for class, want := range cases {
		if got := iconForEntry([]string{class}); got.Value != want {
			t.Fatalf("iconForEntry(%q) = %q, want %q", class, got.Value, want)
		}
	}
}

func TestEntryAddUsesStructuredObjectClassAndAttributeFields(t *testing.T) {
	var schema *plugin.Schema
	for _, r := range New().Routes() {
		if r.ID == "ldap.entry.add" {
			schema = r.Input
		}
	}
	if schema == nil {
		t.Fatal("ldap.entry.add has no input schema")
	}
	var attributes *plugin.Field
	var objectClass *plugin.Field
	for _, g := range schema.Groups {
		for i := range g.Fields {
			if g.Fields[i].Key == "attributes" {
				attributes = &g.Fields[i]
			}
			if g.Fields[i].Key == "object_class" {
				objectClass = &g.Fields[i]
			}
		}
	}
	if objectClass == nil || objectClass.Type != plugin.FieldArray || objectClass.Item == nil || objectClass.Item.Type != plugin.FieldAutocomplete {
		t.Fatalf("object_class field = %#v, want array of autocomplete values", objectClass)
	}
	if attributes == nil {
		t.Fatal("no attributes field")
	}
	if attributes.Type != plugin.FieldMap {
		t.Fatalf("attributes is %q, want map", attributes.Type)
	}
	if attributes.Item == nil || attributes.Item.Type != plugin.FieldArray {
		t.Fatalf("attributes value item is not an array")
	}
	if attributes.Item.Item == nil || attributes.Item.Item.Type != plugin.FieldText {
		t.Fatalf("attributes value array element is not text")
	}
}

func TestStringListAcceptsLegacyCommaStringAndArrayValues(t *testing.T) {
	if got := stringList("top, inetOrgPerson"); !reflect.DeepEqual(got, []string{"top", "inetOrgPerson"}) {
		t.Fatalf("comma list = %#v", got)
	}
	if got := stringList([]any{"top", "person"}); !reflect.DeepEqual(got, []string{"top", "person"}) {
		t.Fatalf("array list = %#v", got)
	}
}
