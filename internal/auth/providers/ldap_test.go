package providers

import (
	"testing"

	"github.com/go-ldap/ldap/v3"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestNewLDAPAuthenticatorValidatesConfig(t *testing.T) {
	_, err := NewLDAPAuthenticator(models.LDAPConfig{}, LDAPAuthenticatorOptions{})
	if err == nil || err.Error() != "ldap provider: host is required" {
		t.Fatalf("expected host validation error, got %v", err)
	}

	_, err = NewLDAPAuthenticator(models.LDAPConfig{
		Host: "ldap.example.com",
	}, LDAPAuthenticatorOptions{})
	if err == nil || err.Error() != "ldap provider: port must be positive" {
		t.Fatalf("expected port validation error, got %v", err)
	}

	_, err = NewLDAPAuthenticator(models.LDAPConfig{
		Host: "ldap.example.com",
		Port: 389,
	}, LDAPAuthenticatorOptions{})
	if err == nil || err.Error() != "ldap provider: base dn is required" {
		t.Fatalf("expected base dn validation error, got %v", err)
	}
}

func TestBuildLDAPFilter(t *testing.T) {
	filter := buildLDAPFilter("", "user@example.com")
	if filter != "(uid=user@example.com)" {
		t.Fatalf("default filter mismatch: %s", filter)
	}

	template := "(&(objectClass=person)(|(uid={identifier})(mail={email})))"
	filter = buildLDAPFilter(template, "alice@example.com")
	if filter != "(&(objectClass=person)(|(uid=alice@example.com)(mail=alice@example.com)))" {
		t.Fatalf("template expansion mismatch: %s", filter)
	}
}

func TestBuildAttributeList(t *testing.T) {
	mapping := map[string]string{
		"email":      "mail",
		"display":    "displayName",
		"username":   "uid",
		"empty":      "   ",
		"duplicates": "uid",
	}

	attrs := buildAttributeList(mapping)
	want := map[string]struct{}{
		"dn":          {},
		"mail":        {},
		"displayName": {},
		"uid":         {},
	}
	if len(attrs) != len(want) {
		t.Fatalf("expected %d attrs, got %d (%v)", len(want), len(attrs), attrs)
	}
	for _, attr := range attrs {
		if _, ok := want[attr]; !ok {
			t.Fatalf("unexpected attribute %s", attr)
		}
	}
}

func TestEntryAttributes(t *testing.T) {
	entry := &ldap.Entry{
		Attributes: []*ldap.EntryAttribute{
			{Name: "mail", Values: []string{"user@example.com"}},
			{Name: "displayName", Values: []string{"Alice"}},
			{Name: "memberOf", Values: []string{"group1", "group2"}},
		},
	}

	attrs := entryAttributes(entry)
	if attrs["mail"][0] != "user@example.com" {
		t.Fatalf("unexpected mail attribute: %v", attrs["mail"])
	}
	if len(attrs["memberOf"]) != 2 {
		t.Fatalf("expected 2 group values, got %v", attrs["memberOf"])
	}
}
