package ldap

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register LDAP plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
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
	if err := plugin.Validate(m, New().Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
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
	if got := searchFilter(""); got != "(objectClass=*)" {
		t.Fatalf("empty filter = %q", got)
	}
	if got := searchFilter("(uid=jdoe)"); got != "(uid=jdoe)" {
		t.Fatalf("raw filter should pass through, got %q", got)
	}
	got := searchFilter("ada")
	if !strings.Contains(got, "(cn=*ada*)") || !strings.HasPrefix(got, "(|") {
		t.Fatalf("free-text filter = %q", got)
	}
	// Injection metacharacters must be escaped, not passed raw.
	if strings.Contains(searchFilter("a)(uid=*"), "a)(uid=*") {
		t.Fatalf("filter value was not escaped: %q", searchFilter("a)(uid=*"))
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

func TestEntryAddAttributesIsMapOfTextArray(t *testing.T) {
	var schema *plugin.Schema
	for _, r := range New().Routes() {
		if r.ID == "ldap.entry.add" {
			schema = r.Input
		}
	}
	if schema == nil {
		t.Fatal("ldap.entry.add has no input schema")
	}
	var field *plugin.Field
	for _, g := range schema.Groups {
		for i := range g.Fields {
			if g.Fields[i].Key == "attributes" {
				field = &g.Fields[i]
			}
		}
	}
	if field == nil {
		t.Fatal("no attributes field")
	}
	if field.Type != plugin.FieldMap {
		t.Fatalf("attributes is %q, want map", field.Type)
	}
	if field.Item == nil || field.Item.Type != plugin.FieldArray {
		t.Fatalf("attributes value item is not an array")
	}
	if field.Item.Item == nil || field.Item.Item.Type != plugin.FieldText {
		t.Fatalf("attributes value array element is not text")
	}
}
