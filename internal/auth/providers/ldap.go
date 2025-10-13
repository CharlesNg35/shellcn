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

	conn, err := a.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	searchFilter := buildLDAPFilter(a.cfg.UserFilter, identifier)
	attributes := buildAttributeList(a.cfg.AttributeMapping)

	searchRequest := ldap.NewSearchRequest(
		a.userBaseDN(),
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

	identity := a.identityFromEntry(userEntry)

	groups, err := a.fetchGroups(conn, userEntry.DN)
	if err != nil {
		return nil, err
	}
	identity.Groups = mergeLDAPGroups(identity.Groups, groups)

	return identity, nil
}

// ListIdentities retrieves all directory entries matching the configured user filter.
func (a *LDAPAuthenticator) ListIdentities(ctx context.Context) ([]*Identity, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	conn, err := a.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	filter := buildLDAPWildcardFilter(a.cfg.UserFilter)
	attributes := buildAttributeList(a.cfg.AttributeMapping)

	searchRequest := ldap.NewSearchRequest(
		a.userBaseDN(),
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		attributes,
		nil,
	)

	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("ldap provider: search: %w", err)
	}

	identities := make([]*Identity, 0, len(searchResult.Entries))
	for _, entry := range searchResult.Entries {
		identity := a.identityFromEntry(entry)
		groups, err := a.fetchGroups(conn, entry.DN)
		if err != nil {
			return nil, err
		}
		identity.Groups = mergeLDAPGroups(identity.Groups, groups)
		identities = append(identities, identity)
	}

	return identities, nil
}

func (a *LDAPAuthenticator) connect() (*ldap.Conn, error) {
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

	if strings.TrimSpace(a.cfg.BindDN) != "" {
		if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("ldap provider: bind service account: %w", err)
		}
	}

	return conn, nil
}

func (a *LDAPAuthenticator) identityFromEntry(entry *ldap.Entry) *Identity {
	attrs := entryAttributes(entry)

	identity := &Identity{
		Provider:      "ldap",
		Subject:       entry.DN,
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

	return identity
}

func (a *LDAPAuthenticator) userBaseDN() string {
	if base := strings.TrimSpace(a.cfg.UserBaseDN); base != "" {
		return base
	}
	return strings.TrimSpace(a.cfg.BaseDN)
}

func (a *LDAPAuthenticator) groupBaseDN() string {
	if base := strings.TrimSpace(a.cfg.GroupBaseDN); base != "" {
		return base
	}
	return a.userBaseDN()
}

func (a *LDAPAuthenticator) fetchGroups(conn *ldap.Conn, userDN string) ([]string, error) {
	if !a.cfg.SyncGroups {
		return nil, nil
	}
	base := a.groupBaseDN()
	if base == "" {
		return nil, errors.New("ldap provider: group base dn is required when sync_groups is enabled")
	}
	memberAttr := strings.TrimSpace(a.cfg.GroupMemberAttribute)
	if memberAttr == "" {
		memberAttr = "member"
	}
	nameAttr := strings.TrimSpace(a.cfg.GroupNameAttribute)
	if nameAttr == "" {
		nameAttr = "cn"
	}
	filter := strings.TrimSpace(a.cfg.GroupFilter)
	if filter == "" {
		filter = "(objectClass=nestedGroup)"
	}
	escapedDN := ldap.EscapeFilter(userDN)
	combinedFilter := fmt.Sprintf("(&%s(%s=%s))", filter, memberAttr, escapedDN)

	attrs := []string{nameAttr, "dn"}
	searchRequest := ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		combinedFilter,
		attrs,
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("ldap provider: fetch groups: %w", err)
	}

	groups := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		attrs := entryAttributes(entry)
		value := strings.TrimSpace(entry.DN)
		if value == "" {
			continue
		}
		name := strings.TrimSpace(attributeLookup(attrs, nameAttr))
		if name != "" {
			groups = append(groups, fmt.Sprintf("%s|%s", name, value))
		} else {
			groups = append(groups, value)
		}
	}

	return groups, nil
}

func mergeLDAPGroups(existing, additional []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(additional))
	merged := make([]string, 0, len(existing)+len(additional))
	for _, value := range existing {
		clean := strings.TrimSpace(value)
		key := strings.ToLower(clean)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, clean)
	}
	for _, value := range additional {
		clean := strings.TrimSpace(value)
		key := strings.ToLower(clean)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, clean)
	}
	return merged
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

func buildLDAPWildcardFilter(template string) string {
	trimmed := strings.TrimSpace(template)
	if trimmed == "" {
		return "(uid=*)"
	}
	filter := strings.ReplaceAll(trimmed, "{identifier}", "*")
	filter = strings.ReplaceAll(filter, "{username}", "*")
	filter = strings.ReplaceAll(filter, "{email}", "*")
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
