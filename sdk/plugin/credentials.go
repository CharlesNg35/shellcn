package plugin

import (
	"fmt"
	"sort"
	"strings"
)

// CredentialKindInfo is the public metadata for a reusable credential kind.
// It declares the credential form the control-plane UI renders.
type CredentialKindInfo struct {
	Kind                CredentialKind `json:"kind"`
	Label               string         `json:"label"`
	Fields              []Field        `json:"fields"`
	CompatibleProtocols []string       `json:"compatibleProtocols,omitempty"`
}

// CredentialPublicField marks a credential field as non-secret metadata that
// can be returned in credential lists and selectors.
func CredentialPublicField(field Field) Field {
	field.Secret = false
	field.Public = true
	return field
}

// CredentialSecretField marks a credential field as secret material.
func CredentialSecretField(field Field) Field {
	field.Secret = true
	field.Public = false
	return field
}

// CredentialKindCatalog resolves reusable credential kind metadata.
type CredentialKindCatalog interface {
	CredentialKinds() []CredentialKindInfo
	CredentialKindLookup(kind CredentialKind) (CredentialKindInfo, bool)
	CredentialKindSupportsProtocol(kind CredentialKind, protocol string) bool
}

var builtInCredentialKindCatalog = []CredentialKindInfo{
	{
		Kind: CredentialKindSSHPrivateKey, Label: "SSH private key",
		Fields: []Field{
			CredentialPublicField(Field{Key: "username", Label: "Username", Type: FieldText, Required: true}),
			CredentialSecretField(Field{Key: "private_key", Label: "Private key", Type: FieldTextarea, Required: true}),
			CredentialSecretField(Field{Key: "passphrase", Label: "Key passphrase", Type: FieldPassword}),
		},
	},
	{
		Kind: CredentialKindSSHPassword, Label: "SSH password",
		Fields: []Field{
			CredentialPublicField(Field{Key: "username", Label: "Username", Type: FieldText, Required: true}),
			CredentialSecretField(Field{Key: "password", Label: "Password", Type: FieldPassword, Required: true}),
		},
	},
	{
		Kind: CredentialKindDBPassword, Label: "Database password",
		Fields: []Field{
			CredentialPublicField(Field{Key: "username", Label: "Database user", Type: FieldText}),
			CredentialSecretField(Field{Key: "password", Label: "Password", Type: FieldPassword, Required: true}),
		},
	},
	{
		Kind: CredentialKindAPIToken, Label: "API token",
		Fields: []Field{
			CredentialPublicField(Field{Key: "subject", Label: "Token name / subject", Type: FieldText}),
			CredentialSecretField(Field{Key: "token", Label: "Token", Type: FieldPassword, Required: true}),
		},
	},
	{
		Kind: CredentialKindTLSClientCert, Label: "TLS client certificate",
		Fields: []Field{
			CredentialPublicField(Field{Key: "subject", Label: "Certificate subject / username", Type: FieldText}),
			CredentialSecretField(Field{Key: "certificate", Label: "Client certificate", Type: FieldTextarea, Required: true}),
			CredentialSecretField(Field{Key: "private_key", Label: "Private key", Type: FieldTextarea, Required: true}),
			CredentialSecretField(Field{Key: "passphrase", Label: "Private key passphrase", Type: FieldPassword}),
		},
	},
	{
		Kind: CredentialKindCloudAccessKey, Label: "Cloud access key",
		Fields: []Field{
			CredentialPublicField(Field{Key: "access_key_id", Label: "Access key ID", Type: FieldText, Required: true}),
			CredentialSecretField(Field{Key: "secret_access_key", Label: "Secret access key", Type: FieldPassword, Required: true}),
			CredentialSecretField(Field{Key: "session_token", Label: "Session token", Type: FieldPassword}),
		},
	},
	{
		Kind: CredentialKindBasicAuth, Label: "Basic auth",
		Fields: []Field{
			CredentialPublicField(Field{Key: "username", Label: "Username", Type: FieldText, Required: true}),
			CredentialSecretField(Field{Key: "password", Label: "Password", Type: FieldPassword, Required: true}),
		},
	},
	{
		Kind: CredentialKindBearerToken, Label: "Bearer token",
		Fields: []Field{
			CredentialPublicField(Field{Key: "subject", Label: "Token name / subject", Type: FieldText}),
			CredentialSecretField(Field{Key: "token", Label: "Token", Type: FieldPassword, Required: true}),
		},
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
		for _, protocol := range protocols {
			catalog.AddSupport(field.Credential.Kind, protocol)
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
	if len(info.Fields) == 0 {
		return fmt.Errorf("credential kind %q is missing Fields", info.Kind)
	}
	if strings.ContainsAny(string(info.Kind), " \t\r\n") {
		return fmt.Errorf("credential kind %q must not contain whitespace", info.Kind)
	}
	seenFields := map[string]bool{}
	hasSecret := false
	for _, field := range info.Fields {
		field.Key = strings.TrimSpace(field.Key)
		field.Label = strings.TrimSpace(field.Label)
		if field.Key == "" {
			return fmt.Errorf("credential kind %q has a field without Key", info.Kind)
		}
		if field.Label == "" {
			return fmt.Errorf("credential kind %q field %q is missing Label", info.Kind, field.Key)
		}
		if field.Type == "" {
			return fmt.Errorf("credential kind %q field %q is missing Type", info.Kind, field.Key)
		}
		if !credentialFieldTypeAllowed(field.Type) {
			return fmt.Errorf("credential kind %q field %q has unsupported Type %q", info.Kind, field.Key, field.Type)
		}
		if err := validateCredentialFieldSubset(info.Kind, field); err != nil {
			return err
		}
		if strings.ContainsAny(field.Key, " \t\r\n.") {
			return fmt.Errorf("credential kind %q field %q has an invalid Key", info.Kind, field.Key)
		}
		if seenFields[field.Key] {
			return fmt.Errorf("credential kind %q has duplicate field %q", info.Kind, field.Key)
		}
		seenFields[field.Key] = true
		if field.Secret {
			hasSecret = true
		}
		if field.Secret && field.Public {
			return fmt.Errorf("credential kind %q field %q cannot be both secret and public", info.Kind, field.Key)
		}
		if !field.Secret && !field.Public {
			return fmt.Errorf("credential kind %q field %q must be either secret or public", info.Kind, field.Key)
		}
	}
	if !hasSecret {
		return fmt.Errorf("credential kind %q must declare at least one secret field", info.Kind)
	}
	if len(info.CompatibleProtocols) > 0 {
		return fmt.Errorf("credential kind %q must not declare CompatibleProtocols; protocol support is derived from credential_ref selectors", info.Kind)
	}
	return nil
}

func credentialFieldTypeAllowed(t FieldType) bool {
	switch t {
	case FieldText, FieldPassword, FieldTextarea:
		return true
	default:
		return false
	}
}

func validateCredentialFieldSubset(kind CredentialKind, field Field) error {
	if field.Default != nil ||
		len(field.Options) > 0 ||
		field.OptionsSource != nil ||
		field.Credential != nil ||
		field.VisibleWhen != nil ||
		len(field.Validators) > 0 ||
		field.Step != nil ||
		len(field.Fields) > 0 ||
		field.Item != nil ||
		field.MinItems != 0 ||
		field.MaxItems != 0 ||
		field.ItemLabel != "" ||
		field.AddLabel != "" ||
		field.KeyLabel != "" ||
		field.KeyPlaceholder != "" {
		return fmt.Errorf("credential kind %q field %q uses unsupported schema features", kind, field.Key)
	}
	return nil
}

func normalizeCredentialKindInfo(info CredentialKindInfo) CredentialKindInfo {
	info.Kind = CredentialKind(strings.TrimSpace(string(info.Kind)))
	info.Label = strings.TrimSpace(info.Label)
	for i := range info.Fields {
		info.Fields[i].Key = strings.TrimSpace(info.Fields[i].Key)
		info.Fields[i].Label = strings.TrimSpace(info.Fields[i].Label)
		info.Fields[i].Placeholder = strings.TrimSpace(info.Fields[i].Placeholder)
		info.Fields[i].Help = strings.TrimSpace(info.Fields[i].Help)
	}
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
	info.Fields = append([]Field(nil), info.Fields...)
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
