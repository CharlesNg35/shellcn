package plugin

import (
	"fmt"
	"sort"
	"strings"
)

// CredentialKindInfo is the public metadata for a reusable credential kind.
// It describes how the control-plane UI should label non-secret fields and
// which protocols can consume the kind.
type CredentialKindInfo struct {
	Kind                CredentialKind `json:"kind"`
	Label               string         `json:"label"`
	SecretLabel         string         `json:"secretLabel"`
	SecretMultiline     bool           `json:"secretMultiline,omitempty"`
	IdentityLabel       string         `json:"identityLabel,omitempty"`
	CompatibleProtocols []string       `json:"compatibleProtocols,omitempty"`
}

// CredentialKindCatalog resolves reusable credential kind metadata.
type CredentialKindCatalog interface {
	CredentialKinds() []CredentialKindInfo
	CredentialKindLookup(kind CredentialKind) (CredentialKindInfo, bool)
	CredentialKindSupportsProtocol(kind CredentialKind, protocol string) bool
}

var builtInCredentialKindCatalog = []CredentialKindInfo{
	{
		Kind: CredentialDBPassword, Label: "Database password", SecretLabel: "Password",
		IdentityLabel: "Database user",
	},
	{
		Kind: CredentialAPIToken, Label: "API token", SecretLabel: "Token",
		IdentityLabel: "Token name / subject",
	},
	{
		Kind: CredentialTLSClientCert, Label: "TLS client certificate", SecretLabel: "Certificate and private key",
		SecretMultiline: true, IdentityLabel: "Certificate subject / username",
	},
	{
		Kind: CredentialCloudAccessKey, Label: "Cloud access key", SecretLabel: "Secret access key",
		IdentityLabel: "Access key ID",
	},
	{
		Kind: CredentialBasicAuth, Label: "Basic auth", SecretLabel: "Password",
		IdentityLabel: "Username",
	},
	{
		Kind: CredentialBearerToken, Label: "Bearer token", SecretLabel: "Token",
		IdentityLabel: "Token name / subject",
	},
}

// CredentialKindSet is a mutable credential-kind catalog.
type CredentialKindSet struct {
	order    []CredentialKind
	byID     map[CredentialKind]CredentialKindInfo
	supports map[CredentialKind]map[string]bool
}

// NewCredentialKindSet returns a catalog initialized with the given credential
// kinds.
func NewCredentialKindSet(base []CredentialKindInfo) (*CredentialKindSet, error) {
	c := &CredentialKindSet{
		byID:     map[CredentialKind]CredentialKindInfo{},
		supports: map[CredentialKind]map[string]bool{},
	}
	for _, info := range base {
		if err := c.add(info); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func mustCredentialKindSet(base []CredentialKindInfo) *CredentialKindSet {
	c, err := NewCredentialKindSet(base)
	if err != nil {
		panic(err)
	}
	return c
}

// MustCredentialKindSet returns a catalog initialized with the given credential
// kinds and panics if the input is invalid.
func MustCredentialKindSet(base []CredentialKindInfo) *CredentialKindSet {
	return mustCredentialKindSet(base)
}

func (c *CredentialKindSet) clone() *CredentialKindSet {
	out := &CredentialKindSet{
		order:    append([]CredentialKind(nil), c.order...),
		byID:     make(map[CredentialKind]CredentialKindInfo, len(c.byID)),
		supports: make(map[CredentialKind]map[string]bool, len(c.supports)),
	}
	for kind, info := range c.byID {
		out.byID[kind] = cloneCredentialKindInfo(info)
	}
	for kind, protocols := range c.supports {
		out.supports[kind] = map[string]bool{}
		for protocol := range protocols {
			out.supports[kind][protocol] = true
		}
	}
	return out
}

// Clone returns a deep copy of the catalog.
func (c *CredentialKindSet) Clone() *CredentialKindSet {
	return c.clone()
}

// cloneWithout copies the set minus the given kinds (used to revalidate a
// plugin update against a catalog that excludes its own old declarations).
func (c *CredentialKindSet) cloneWithout(exclude map[CredentialKind]bool) *CredentialKindSet {
	out := c.clone()
	if len(exclude) == 0 {
		return out
	}
	order := out.order[:0]
	for _, k := range out.order {
		if exclude[k] {
			delete(out.byID, k)
			continue
		}
		order = append(order, k)
	}
	out.order = order
	return out
}

// CloneWithout returns a deep copy excluding the given credential kinds.
func (c *CredentialKindSet) CloneWithout(exclude map[CredentialKind]bool) *CredentialKindSet {
	return c.cloneWithout(exclude)
}

func (c *CredentialKindSet) add(info CredentialKindInfo) error {
	info = normalizeCredentialKindInfo(info)
	if err := validateCredentialKindInfo(info); err != nil {
		return err
	}
	if _, exists := c.byID[info.Kind]; exists {
		return fmt.Errorf("duplicate credential kind %q", info.Kind)
	}
	c.order = append(c.order, info.Kind)
	c.byID[info.Kind] = info
	return nil
}

// Add appends a credential kind to the catalog.
func (c *CredentialKindSet) Add(info CredentialKindInfo) error {
	return c.add(info)
}

func (c *CredentialKindSet) addSupport(kind CredentialKind, protocol string) {
	protocol = strings.TrimSpace(protocol)
	if kind == "" || protocol == "" {
		return
	}
	if c.supports[kind] == nil {
		c.supports[kind] = map[string]bool{}
	}
	c.supports[kind][protocol] = true
}

// AddSupport marks a credential kind as compatible with a protocol.
func (c *CredentialKindSet) AddSupport(kind CredentialKind, protocol string) {
	c.addSupport(kind, protocol)
}

func (c *CredentialKindSet) CredentialKinds() []CredentialKindInfo {
	out := make([]CredentialKindInfo, 0, len(c.order))
	for _, kind := range c.order {
		out = append(out, c.withSupports(kind, c.byID[kind]))
	}
	return out
}

func (c *CredentialKindSet) CredentialKindLookup(kind CredentialKind) (CredentialKindInfo, bool) {
	info, ok := c.byID[kind]
	if !ok {
		return CredentialKindInfo{}, false
	}
	return c.withSupports(kind, info), true
}

func (c *CredentialKindSet) CredentialKindSupportsProtocol(kind CredentialKind, protocol string) bool {
	if _, ok := c.byID[kind]; !ok {
		return false
	}
	if strings.TrimSpace(protocol) == "" {
		return true
	}
	return c.supports[kind][protocol]
}

func (c *CredentialKindSet) withSupports(kind CredentialKind, info CredentialKindInfo) CredentialKindInfo {
	info = cloneCredentialKindInfo(info)
	info.CompatibleProtocols = info.CompatibleProtocols[:0]
	for protocol := range c.supports[kind] {
		info.CompatibleProtocols = append(info.CompatibleProtocols, protocol)
	}
	sort.Strings(info.CompatibleProtocols)
	return info
}

// BuiltInCredentialKinds returns core credential kinds that are intentionally
// shared by multiple protocol families.
func BuiltInCredentialKinds() []CredentialKindInfo {
	return cloneCredentialKindInfos(builtInCredentialKindCatalog)
}

// CredentialKinds returns the core built-in credential-kind catalog.
func CredentialKinds() []CredentialKindInfo {
	return BuiltInCredentialKinds()
}

// CredentialKindLookup returns one core built-in credential kind's metadata.
func CredentialKindLookup(kind CredentialKind) (CredentialKindInfo, bool) {
	return mustCredentialKindSet(builtInCredentialKindCatalog).CredentialKindLookup(kind)
}

// CredentialKindSupportsProtocol reports whether a built-in credential kind has
// built-in protocol support. Core built-ins intentionally declare no protocol
// support. Gateway runtime catalogs derive support from plugin credential_ref
// selectors.
func CredentialKindSupportsProtocol(kind CredentialKind, protocol string) bool {
	return mustCredentialKindSet(builtInCredentialKindCatalog).CredentialKindSupportsProtocol(kind, protocol)
}

func AddCredentialKindSupports(catalog *CredentialKindSet, m Manifest) {
	for _, group := range m.Config.Groups {
		for _, field := range group.Fields {
			addFieldCredentialKindSupports(catalog, m.Name, field)
		}
	}
}

func addFieldCredentialKindSupports(catalog *CredentialKindSet, pluginName string, field Field) {
	if field.Type == FieldCredentialRef && field.Credential != nil {
		protocols := field.Credential.Protocols
		if len(protocols) == 0 && pluginName != "" {
			protocols = []string{pluginName}
		}
		for _, kind := range field.Credential.Kinds {
			for _, protocol := range protocols {
				catalog.AddSupport(kind, protocol)
			}
		}
	}
	for _, child := range field.Fields {
		addFieldCredentialKindSupports(catalog, pluginName, child)
	}
	if field.Item != nil {
		addFieldCredentialKindSupports(catalog, pluginName, *field.Item)
	}
}

func validateCredentialKindInfo(info CredentialKindInfo) error {
	if info.Kind == "" {
		return fmt.Errorf("credential kind is missing Kind")
	}
	if info.Label == "" {
		return fmt.Errorf("credential kind %q is missing Label", info.Kind)
	}
	if info.SecretLabel == "" {
		return fmt.Errorf("credential kind %q is missing SecretLabel", info.Kind)
	}
	if strings.ContainsAny(string(info.Kind), " \t\r\n") {
		return fmt.Errorf("credential kind %q must not contain whitespace", info.Kind)
	}
	if len(info.CompatibleProtocols) > 0 {
		return fmt.Errorf("credential kind %q must not declare CompatibleProtocols; protocol support is derived from credential_ref selectors", info.Kind)
	}
	return nil
}

func normalizeCredentialKindInfo(info CredentialKindInfo) CredentialKindInfo {
	info.Kind = CredentialKind(strings.TrimSpace(string(info.Kind)))
	info.Label = strings.TrimSpace(info.Label)
	info.SecretLabel = strings.TrimSpace(info.SecretLabel)
	info.IdentityLabel = strings.TrimSpace(info.IdentityLabel)
	protocols := make([]string, 0, len(info.CompatibleProtocols))
	seen := map[string]bool{}
	for _, protocol := range info.CompatibleProtocols {
		protocol = strings.TrimSpace(protocol)
		if protocol == "" || seen[protocol] {
			continue
		}
		seen[protocol] = true
		protocols = append(protocols, protocol)
	}
	sort.Strings(protocols)
	info.CompatibleProtocols = protocols
	return info
}

func cloneCredentialKindInfo(info CredentialKindInfo) CredentialKindInfo {
	info.CompatibleProtocols = append([]string(nil), info.CompatibleProtocols...)
	return info
}

func cloneCredentialKindInfos(in []CredentialKindInfo) []CredentialKindInfo {
	out := make([]CredentialKindInfo, 0, len(in))
	for _, info := range in {
		out = append(out, cloneCredentialKindInfo(info))
	}
	return out
}

func credentialKindDefinitions(in []CredentialKindInfo) []CredentialKindInfo {
	out := cloneCredentialKindInfos(in)
	for i := range out {
		out[i].CompatibleProtocols = nil
	}
	return out
}
