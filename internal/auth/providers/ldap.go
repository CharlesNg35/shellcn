package providers

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/charlesng35/shellcn/internal/models"
)

// LDAPAuthenticateInput contains the credentials supplied by the user.
type LDAPAuthenticateInput struct {
	Identifier string
	Password   string
}

// LDAPAuthenticatorOptions configures the LDAP authenticator.
type LDAPAuthenticatorOptions struct {
	Timeout time.Duration
}

// LDAPAuthenticator performs directory binds to validate credentials and collect attributes.
type LDAPAuthenticator struct {
	cfg     models.LDAPConfig
	timeout time.Duration
}

// NewLDAPAuthenticator constructs an authenticator for the provided configuration.
func NewLDAPAuthenticator(cfg models.LDAPConfig, opts LDAPAuthenticatorOptions) (*LDAPAuthenticator, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, errors.New("ldap provider: host is required")
	}
	if cfg.Port <= 0 {
		return nil, errors.New("ldap provider: port must be positive")
	}
	if strings.TrimSpace(cfg.BaseDN) == "" {
		return nil, errors.New("ldap provider: base dn is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &LDAPAuthenticator{cfg: cfg, timeout: timeout}, nil
}

// Authenticate verifies the supplied credentials against the directory and returns the mapped identity.
func (a *LDAPAuthenticator) Authenticate(ctx context.Context, input LDAPAuthenticateInput) (*Identity, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	identifier := strings.TrimSpace(input.Identifier)
	if identifier == "" || input.Password == "" {
		return nil, errors.New("ldap provider: identifier and password are required")
	}

	scheme := "ldap"
	dialOpts := []ldap.DialOpt{ldap.DialWithDialer(&net.Dialer{Timeout: a.timeout})}
	if a.cfg.UseTLS {
		scheme = "ldaps"
		dialOpts = append(dialOpts, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: a.cfg.SkipVerify}))
	}

	conn, err := ldap.DialURL(fmt.Sprintf("%s://%s:%d", scheme, a.cfg.Host, a.cfg.Port), dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("ldap provider: dial: %w", err)
	}
	defer conn.Close()

	if strings.TrimSpace(a.cfg.BindDN) != "" {
		if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap provider: bind service account: %w", err)
		}
	}

	searchFilter := buildLDAPFilter(a.cfg.UserFilter, identifier)
	attributes := buildAttributeList(a.cfg.AttributeMapping)

	searchRequest := ldap.NewSearchRequest(
		a.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		searchFilter,
		attributes,
		nil,
	)

	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("ldap provider: search: %w", err)
	}
	if len(searchResult.Entries) == 0 {
		return nil, errors.New("ldap provider: user not found")
	}
	userEntry := searchResult.Entries[0]

	if err := conn.Bind(userEntry.DN, input.Password); err != nil {
		return nil, errors.New("ldap provider: invalid credentials")
	}

	attrs := entryAttributes(userEntry)

	identity := &Identity{
		Provider:      "ldap",
		Subject:       userEntry.DN,
		Email:         attributeLookup(attrs, a.cfg.AttributeMapping["email"]),
		EmailVerified: true,
		FirstName:     attributeLookup(attrs, a.cfg.AttributeMapping["first_name"]),
		LastName:      attributeLookup(attrs, a.cfg.AttributeMapping["last_name"]),
		DisplayName:   attributeLookup(attrs, a.cfg.AttributeMapping["display_name"]),
		RawClaims:     make(map[string]any, len(attrs)),
	}

	if identity.Email == "" {
		identity.Email = attributeLookup(attrs, "mail")
	}
	if identity.DisplayName == "" {
		identity.DisplayName = attributeLookup(attrs, "displayName")
	}
	if groupsAttr := strings.TrimSpace(a.cfg.AttributeMapping["groups"]); groupsAttr != "" {
		identity.Groups = attrs[groupsAttr]
	}

	for k, v := range attrs {
		values := make([]string, len(v))
		copy(values, v)
		identity.RawClaims[k] = values
	}

	if usernameAttr := strings.TrimSpace(a.cfg.AttributeMapping["username"]); usernameAttr != "" {
		identity.RawClaims["username"] = attributeLookup(attrs, usernameAttr)
	}

	return identity, nil
}

func buildLDAPFilter(template string, identifier string) string {
	if strings.TrimSpace(template) == "" {
		return fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(identifier))
	}
	escaped := ldap.EscapeFilter(identifier)
	filter := strings.ReplaceAll(template, "{identifier}", escaped)
	filter = strings.ReplaceAll(filter, "{username}", escaped)
	filter = strings.ReplaceAll(filter, "{email}", escaped)
	return filter
}

func buildAttributeList(mapping map[string]string) []string {
	attrs := map[string]struct{}{
		"dn":          {},
		"mail":        {},
		"displayName": {},
	}
	for _, v := range mapping {
		if strings.TrimSpace(v) == "" {
			continue
		}
		attrs[strings.TrimSpace(v)] = struct{}{}
	}
	list := make([]string, 0, len(attrs))
	for k := range attrs {
		list = append(list, k)
	}
	return list
}

func entryAttributes(entry *ldap.Entry) map[string][]string {
	result := make(map[string][]string, len(entry.Attributes))
	for _, attr := range entry.Attributes {
		values := make([]string, len(attr.Values))
		copy(values, attr.Values)
		result[attr.Name] = values
	}
	return result
}
