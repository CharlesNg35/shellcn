package service

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/secrets"
	"github.com/charlesng/shellcn/internal/store"
)

// ConnectionService owns the control-plane lifecycle of a connection: it
// validates submitted config against the plugin's schema, splits secret from
// non-secret fields, encrypts secrets before the store, and enforces write-only
// secret semantics on update. Secret values never travel back toward the client.
type ConnectionService struct {
	conns   store.ConnectionStore
	plugins *plugin.Registry
	creds   *CredentialService
	vault   secrets.SecretStore
}

// NewConnectionService wires the dependencies.
func NewConnectionService(conns store.ConnectionStore, plugins *plugin.Registry, creds *CredentialService, vault secrets.SecretStore) *ConnectionService {
	return &ConnectionService{conns: conns, plugins: plugins, creds: creds, vault: vault}
}

// ConnectionInput is a create/update request. Config carries the full submitted
// form values: non-secret fields, inline secret values, and credential refs.
type ConnectionInput struct {
	Name      string
	Protocol  string
	Transport string
	Config    map[string]any
	// Recording is the per-class policy (class -> disabled|manual|auto). A nil map
	// on update preserves the stored policy; on create it means recording is off.
	Recording map[string]string
}

// ConnectionDetail is the edit/detail read: non-secret config plus a per-secret
// field presence map ("set" / "not set"). It never carries secret values.
type ConnectionDetail struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Protocol  string            `json:"protocol"`
	Transport string            `json:"transport"`
	OwnerID   string            `json:"ownerId"`
	Config    map[string]any    `json:"config"`
	Secrets   map[string]string `json:"secrets"`
	Recording map[string]string `json:"recording"`
}

// secretPlaceholder stands in for a retained (untouched) secret while validating
// an update, so a required secret field does not fail when left blank.
const secretPlaceholder = "\x00kept\x00"

// Create validates the input against the plugin schema, encrypts inline secrets,
// and persists a new connection owned by ownerID.
func (s *ConnectionService) Create(ctx context.Context, ownerID string, in ConnectionInput) (models.Connection, error) {
	m, ok := s.plugins.Manifest(in.Protocol)
	if !ok {
		return models.Connection{}, fmt.Errorf("%w: unknown protocol %q", plugin.ErrInvalidInput, in.Protocol)
	}
	transport, err := resolveTransport(m, in.Transport)
	if err != nil {
		return models.Connection{}, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return models.Connection{}, fmt.Errorf("%w: name is required", plugin.ErrInvalidInput)
	}
	if err := m.Config.ValidateValues(in.Config, nil); err != nil {
		return models.Connection{}, err
	}
	if err := s.checkCredentialRefs(ctx, ownerID, in.Protocol, m.Config, in.Config); err != nil {
		return models.Connection{}, err
	}
	recording, err := resolveRecordingPolicy(m, in.Recording, nil)
	if err != nil {
		return models.Connection{}, err
	}

	config, plain := splitSecrets(m.Config, in.Config)
	enc, err := secrets.EncryptMap(ctx, s.vault, plain)
	if err != nil {
		return models.Connection{}, fmt.Errorf("encrypt secrets: %w", err)
	}

	now := time.Now()
	conn := models.Connection{
		ID:        uuid.NewString(),
		Name:      in.Name,
		Protocol:  in.Protocol,
		OwnerID:   ownerID,
		Transport: transport,
		Config:    config,
		Secrets:   enc,
		Recording: recording,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.conns.Create(ctx, &conn); err != nil {
		return models.Connection{}, err
	}
	return conn, nil
}

// Update re-validates and persists changes to an existing connection. Secret
// fields follow write-only rules: a blank/omitted value keeps the stored
// ciphertext; a non-blank value replaces it.
func (s *ConnectionService) Update(ctx context.Context, existing models.Connection, in ConnectionInput) (models.Connection, error) {
	m, ok := s.plugins.Manifest(existing.Protocol)
	if !ok {
		return models.Connection{}, fmt.Errorf("%w: unknown protocol %q", plugin.ErrInvalidInput, existing.Protocol)
	}
	transport, err := resolveTransport(m, in.Transport)
	if err != nil {
		return models.Connection{}, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return models.Connection{}, fmt.Errorf("%w: name is required", plugin.ErrInvalidInput)
	}

	// Validate against a view where retained secrets count as present.
	validateView := map[string]any{}
	maps.Copy(validateView, in.Config)
	for _, key := range secretKeys(m.Config) {
		if isBlank(validateView[key]) && len(existing.Secrets[key]) > 0 {
			validateView[key] = secretPlaceholder
		}
	}
	if err := m.Config.ValidateValues(validateView, nil); err != nil {
		return models.Connection{}, err
	}
	if err := s.checkCredentialRefs(ctx, existing.OwnerID, existing.Protocol, m.Config, in.Config); err != nil {
		return models.Connection{}, err
	}
	recording, err := resolveRecordingPolicy(m, in.Recording, existing.Recording)
	if err != nil {
		return models.Connection{}, err
	}

	config, plain := splitSecrets(m.Config, in.Config)
	enc := map[string][]byte{}
	for _, key := range secretKeys(m.Config) {
		if v, ok := plain[key]; ok {
			ct, err := s.vault.Encrypt(ctx, []byte(v))
			if err != nil {
				return models.Connection{}, fmt.Errorf("encrypt secrets: %w", err)
			}
			enc[key] = ct
		} else if prev, ok := existing.Secrets[key]; ok {
			enc[key] = prev
		}
	}

	existing.Name = in.Name
	existing.Transport = transport
	existing.Config = config
	existing.Secrets = enc
	existing.Recording = recording
	existing.UpdatedAt = time.Now()
	if err := s.conns.Update(ctx, &existing); err != nil {
		return models.Connection{}, err
	}
	return existing, nil
}

// Delete removes a connection.
func (s *ConnectionService) Delete(ctx context.Context, id string) error {
	return s.conns.Delete(ctx, id)
}

// ReferencesCredential reports whether any connection references credentialID
// through its config (the control plane uses this to block credential deletion).
func (s *ConnectionService) ReferencesCredential(ctx context.Context, credentialID string) (bool, error) {
	conns, err := s.conns.List(ctx)
	if err != nil {
		return false, err
	}
	for _, c := range conns {
		if m, ok := s.plugins.Manifest(c.Protocol); ok {
			for _, key := range credentialRefKeys(m.Config) {
				if id, _ := c.Config[key].(string); id == credentialID {
					return true, nil
				}
			}
			continue
		}
		if id, _ := c.Config[CredentialField].(string); id == credentialID {
			return true, nil
		}
	}
	return false, nil
}

// Detail projects a connection to its non-secret edit view, marking each secret
// field as "set" or "not set" without revealing any value.
func (s *ConnectionService) Detail(conn models.Connection) ConnectionDetail {
	m, _ := s.plugins.Manifest(conn.Protocol)
	state := map[string]string{}
	for _, key := range secretKeys(m.Config) {
		state[key] = secrets.State(len(conn.Secrets[key]) > 0)
	}
	config := conn.Config
	if config == nil {
		config = map[string]any{}
	}
	recording := conn.Recording
	if recording == nil {
		recording = map[string]string{}
	}
	return ConnectionDetail{
		ID: conn.ID, Name: conn.Name, Protocol: conn.Protocol,
		Transport: conn.Transport, OwnerID: conn.OwnerID,
		Config: config, Secrets: state, Recording: recording,
	}
}

// resolveRecordingPolicy validates a submitted per-class policy against the
// plugin's declared recording classes. A nil submitted map preserves prior (used
// on update so a projection refresh never silently changes recording). Only
// supported classes are kept, and unsupported classes or policies are rejected.
func resolveRecordingPolicy(m plugin.Manifest, submitted, prior map[string]string) (map[string]string, error) {
	if submitted == nil {
		return prior, nil
	}
	out := map[string]string{}
	for class, pol := range submitted {
		rc := plugin.RecordingClass(class)
		if !m.SupportsRecordingClass(rc) {
			return nil, fmt.Errorf("%w: plugin %q does not support recording class %q", plugin.ErrInvalidInput, m.Name, class)
		}
		if !plugin.ValidRecordingPolicy(plugin.RecordingPolicy(pol)) {
			return nil, fmt.Errorf("%w: invalid recording policy %q", plugin.ErrInvalidInput, pol)
		}
		// Disabled is the default; storing it is redundant noise.
		if plugin.RecordingPolicy(pol) != plugin.PolicyDisabled {
			out[class] = pol
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// checkCredentialRefs ensures every referenced credential is usable by ownerID
// and matches the field's selector constraints.
func (s *ConnectionService) checkCredentialRefs(ctx context.Context, ownerID, protocol string, schema plugin.Schema, values map[string]any) error {
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Type != plugin.FieldCredentialRef {
				continue
			}
			id, _ := values[field.Key].(string)
			if strings.TrimSpace(id) == "" {
				continue
			}
			kinds := credentialSelectorKinds(field.Credential)
			if field.Credential != nil && len(field.Credential.Protocols) > 0 && !slices.Contains(field.Credential.Protocols, protocol) {
				return fmt.Errorf("%w: credential field %q is not valid for protocol %q", plugin.ErrInvalidInput, field.Key, protocol)
			}
			if err := s.creds.EnsureUsableFor(ctx, ownerID, id, kinds, protocol); err != nil {
				return err
			}
		}
	}
	return nil
}

func credentialRefKeys(schema plugin.Schema) []string {
	var keys []string
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Type == plugin.FieldCredentialRef {
				keys = append(keys, field.Key)
			}
		}
	}
	return keys
}

func credentialSelectorKinds(selector *plugin.CredentialSelector) []string {
	if selector == nil {
		return nil
	}
	kinds := make([]string, 0, len(selector.Kinds))
	for _, kind := range selector.Kinds {
		kinds = append(kinds, string(kind))
	}
	return kinds
}

// resolveTransport defaults to direct and rejects unsupported transports.
func resolveTransport(m plugin.Manifest, requested string) (string, error) {
	if requested == "" {
		return string(plugin.TransportDirect), nil
	}
	t := plugin.Transport(requested)
	if !m.SupportsTransport(t) {
		return "", fmt.Errorf("%w: transport %q is not supported by %q", plugin.ErrInvalidInput, requested, m.Name)
	}
	return requested, nil
}

// secretKeys returns the keys of all Secret==true fields in a schema.
func secretKeys(schema plugin.Schema) []string {
	var keys []string
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Secret {
				keys = append(keys, field.Key)
			}
		}
	}
	return keys
}

// splitSecrets separates a submitted value map into the non-secret config to
// store as plaintext and the non-blank secret values to encrypt.
func splitSecrets(schema plugin.Schema, in map[string]any) (config map[string]any, plain map[string]string) {
	secret := map[string]bool{}
	known := map[string]bool{}
	for _, key := range secretKeys(schema) {
		secret[key] = true
	}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			known[field.Key] = true
		}
	}
	config = map[string]any{}
	plain = map[string]string{}
	for k, v := range in {
		if !known[k] {
			continue
		}
		if secret[k] {
			if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
				plain[k] = str
			}
			continue
		}
		config[k] = v
	}
	return config, plain
}

func isBlank(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) == ""
}
