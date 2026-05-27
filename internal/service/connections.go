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
	ActorID   string
	// PreserveCredentials names credential_ref fields whose existing stored
	// credential ID must be kept because the editor cannot read that credential.
	PreserveCredentials []string
	// Recording is the per-class policy (class -> disabled|manual|auto). A nil map
	// on update preserves the stored policy; on create it means recording is off.
	Recording map[string]string
}

// ConnectionFolderInput is a sidebar folder create/update request.
type ConnectionFolderInput struct {
	Name     string
	Color    string
	ParentID string
}

// ConnectionFolderDTO is the client-facing folder record.
type ConnectionFolderDTO struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId,omitempty"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
}

// ConnectionPlacementInput is one row of the user's sidebar layout.
type ConnectionPlacementInput struct {
	ConnectionID string `json:"connectionId"`
	FolderID     string `json:"folderId"`
	SortOrder    int    `json:"sortOrder"`
}

// ConnectionFolderOrderInput is one sidebar folder position.
type ConnectionFolderOrderInput struct {
	FolderID  string `json:"folderId"`
	ParentID  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
}

// ConnectionDetail is the edit/detail read: non-secret config plus a per-secret
// field presence map ("set" / "not set"). It never carries secret values.
type ConnectionDetail struct {
	ID          string                        `json:"id"`
	Name        string                        `json:"name"`
	Protocol    string                        `json:"protocol"`
	Transport   string                        `json:"transport"`
	OwnerID     string                        `json:"ownerId"`
	Config      map[string]any                `json:"config"`
	Secrets     map[string]string             `json:"secrets"`
	Credentials map[string]CredentialRefState `json:"credentials,omitempty"`
	Recording   map[string]string             `json:"recording"`
}

type CredentialRefState struct {
	State    string                    `json:"state"`
	Readable bool                      `json:"readable"`
	Summary  *models.CredentialSummary `json:"summary,omitempty"`
}

// secretPlaceholder stands in for a retained (untouched) secret while validating
// an update, so a required secret field does not fail when left blank.
const secretPlaceholder = "\x00kept\x00"

var folderColors = map[string]bool{
	"slate":   true,
	"blue":    true,
	"teal":    true,
	"emerald": true,
	"amber":   true,
	"rose":    true,
	"violet":  true,
	"cyan":    true,
}

const defaultFolderColor = "slate"

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
	context := connectionSchemaContext(in.Protocol, transport)
	configWithDefaults := m.Config.ValuesWithDefaults(in.Config)
	if err := m.Config.ValidateValuesWithContext(configWithDefaults, nil, context); err != nil {
		return models.Connection{}, err
	}
	visibleConfig := m.Config.VisibleValues(configWithDefaults, context)
	actorID := in.ActorID
	if actorID == "" {
		actorID = ownerID
	}
	if err := s.checkCredentialRefs(ctx, actorID, in.Protocol, m.Config, visibleConfig); err != nil {
		return models.Connection{}, err
	}
	recording, err := resolveRecordingPolicy(m, in.Recording, nil)
	if err != nil {
		return models.Connection{}, err
	}

	config, plain := splitSecrets(m.Config, visibleConfig)
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

	context := connectionSchemaContext(existing.Protocol, transport)
	mergedConfig, err := s.mergePreservedCredentialRefs(existing, m.Config, in)
	if err != nil {
		return models.Connection{}, err
	}
	mergedConfig = m.Config.ValuesWithDefaults(mergedConfig)
	// Validate against a view where retained secrets count as present.
	validateView := map[string]any{}
	maps.Copy(validateView, mergedConfig)
	for _, key := range secretKeys(m.Config) {
		if isBlank(validateView[key]) && len(existing.Secrets[key]) > 0 {
			validateView[key] = secretPlaceholder
		}
	}
	if err := m.Config.ValidateValuesWithContext(validateView, nil, context); err != nil {
		return models.Connection{}, err
	}
	visibleConfig := m.Config.VisibleValues(mergedConfig, context)
	actorID := in.ActorID
	if actorID == "" {
		actorID = existing.OwnerID
	}
	if err := s.checkCredentialRefsForUpdate(ctx, actorID, existing, m.Config, visibleConfig, in.PreserveCredentials); err != nil {
		return models.Connection{}, err
	}
	recording, err := resolveRecordingPolicy(m, in.Recording, existing.Recording)
	if err != nil {
		return models.Connection{}, err
	}

	config, plain := splitSecrets(m.Config, visibleConfig)
	enc := map[string][]byte{}
	for _, key := range m.Config.VisibleSecretKeys(validateView, context) {
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

// CreateFolder creates one user-owned sidebar folder.
func (s *ConnectionService) CreateFolder(ctx context.Context, folders store.ConnectionFolderStore, userID string, in ConnectionFolderInput) (models.ConnectionFolder, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.ConnectionFolder{}, fmt.Errorf("%w: folder name is required", plugin.ErrInvalidInput)
	}
	color, err := resolveFolderColor(in.Color)
	if err != nil {
		return models.ConnectionFolder{}, err
	}
	existing, err := folders.ListByUser(ctx, userID)
	if err != nil {
		return models.ConnectionFolder{}, err
	}
	if in.ParentID != "" && !folderExists(existing, in.ParentID) {
		return models.ConnectionFolder{}, fmt.Errorf("%w: unknown parent folder %q", plugin.ErrInvalidInput, in.ParentID)
	}
	now := time.Now()
	folder := models.ConnectionFolder{
		ID: uuid.NewString(), UserID: userID, Name: name, Color: color,
		ParentID: in.ParentID, SortOrder: nextFolderOrder(existing, in.ParentID), CreatedAt: now, UpdatedAt: now,
	}
	if err := folders.Create(ctx, &folder); err != nil {
		return models.ConnectionFolder{}, err
	}
	return folder, nil
}

// UpdateFolder updates a user-owned sidebar folder.
func (s *ConnectionService) UpdateFolder(ctx context.Context, folders store.ConnectionFolderStore, existing models.ConnectionFolder, in ConnectionFolderInput) (models.ConnectionFolder, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.ConnectionFolder{}, fmt.Errorf("%w: folder name is required", plugin.ErrInvalidInput)
	}
	color, err := resolveFolderColor(in.Color)
	if err != nil {
		return models.ConnectionFolder{}, err
	}
	existing.Name = name
	existing.Color = color
	existing.UpdatedAt = time.Now()
	if err := folders.Update(ctx, &existing); err != nil {
		return models.ConnectionFolder{}, err
	}
	return existing, nil
}

// FolderDTO projects a folder for the browser.
func FolderDTO(f models.ConnectionFolder) ConnectionFolderDTO {
	return ConnectionFolderDTO{ID: f.ID, ParentID: f.ParentID, Name: f.Name, Color: f.Color, SortOrder: f.SortOrder}
}

// SaveConnectionLayout validates and persists a user's connection placements.
func SaveConnectionLayout(ctx context.Context, placements store.ConnectionPlacementStore, userID string, accessible map[string]bool, folders map[string]bool, in []ConnectionPlacementInput) error {
	seen := map[string]bool{}
	now := time.Now()
	for _, item := range in {
		if !accessible[item.ConnectionID] {
			return fmt.Errorf("%w: connection %q is not accessible", plugin.ErrForbidden, item.ConnectionID)
		}
		if seen[item.ConnectionID] {
			return fmt.Errorf("%w: duplicate connection %q", plugin.ErrInvalidInput, item.ConnectionID)
		}
		seen[item.ConnectionID] = true
		if item.FolderID != "" && !folders[item.FolderID] {
			return fmt.Errorf("%w: unknown folder %q", plugin.ErrInvalidInput, item.FolderID)
		}
		p := models.ConnectionPlacement{
			UserID: userID, ConnectionID: item.ConnectionID, FolderID: item.FolderID,
			SortOrder: item.SortOrder, UpdatedAt: now,
		}
		if err := placements.Set(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}

// SaveConnectionFolderOrder validates and persists a user's folder ordering.
func SaveConnectionFolderOrder(ctx context.Context, folderStore store.ConnectionFolderStore, userID string, in []ConnectionFolderOrderInput) error {
	existing, err := folderStore.ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	byID := map[string]models.ConnectionFolder{}
	for _, folder := range existing {
		byID[folder.ID] = folder
	}
	parentByID := map[string]string{}
	for _, folder := range existing {
		parentByID[folder.ID] = folder.ParentID
	}
	seen := map[string]bool{}
	now := time.Now()
	for _, item := range in {
		_, ok := byID[item.FolderID]
		if !ok {
			return fmt.Errorf("%w: unknown folder %q", plugin.ErrInvalidInput, item.FolderID)
		}
		if seen[item.FolderID] {
			return fmt.Errorf("%w: duplicate folder %q", plugin.ErrInvalidInput, item.FolderID)
		}
		if item.ParentID == item.FolderID {
			return fmt.Errorf("%w: folder cannot contain itself", plugin.ErrInvalidInput)
		}
		if item.ParentID != "" {
			if _, ok := byID[item.ParentID]; !ok {
				return fmt.Errorf("%w: unknown parent folder %q", plugin.ErrInvalidInput, item.ParentID)
			}
		}
		seen[item.FolderID] = true
		parentByID[item.FolderID] = item.ParentID
	}
	if hasFolderCycle(parentByID) {
		return fmt.Errorf("%w: folder nesting contains a cycle", plugin.ErrInvalidInput)
	}
	for _, item := range in {
		folder := byID[item.FolderID]
		folder.ParentID = item.ParentID
		folder.SortOrder = item.SortOrder
		folder.UpdatedAt = now
		if err := folderStore.Update(ctx, &folder); err != nil {
			return err
		}
	}
	return nil
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
			config := m.Config.VisibleValues(
				m.Config.ValuesWithDefaults(c.Config),
				connectionSchemaContext(c.Protocol, c.Transport),
			)
			for _, key := range credentialRefKeys(m.Config) {
				if id, _ := config[key].(string); id == credentialID {
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
func (s *ConnectionService) Detail(ctx context.Context, userID string, conn models.Connection) ConnectionDetail {
	m, _ := s.plugins.Manifest(conn.Protocol)
	state := map[string]string{}
	context := connectionSchemaContext(conn.Protocol, conn.Transport)
	configWithDefaults := m.Config.ValuesWithDefaults(conn.Config)
	for _, key := range m.Config.VisibleSecretKeys(configWithDefaults, context) {
		state[key] = secrets.State(len(conn.Secrets[key]) > 0)
	}
	config := map[string]any{}
	maps.Copy(config, m.Config.VisibleValues(configWithDefaults, context))
	credentialStates := s.credentialRefStates(ctx, userID, m.Config, config)
	recording := conn.Recording
	if recording == nil {
		recording = map[string]string{}
	}
	return ConnectionDetail{
		ID: conn.ID, Name: conn.Name, Protocol: conn.Protocol,
		Transport: conn.Transport, OwnerID: conn.OwnerID,
		Config: config, Secrets: state, Credentials: credentialStates, Recording: recording,
	}
}

func (s *ConnectionService) credentialRefStates(ctx context.Context, userID string, schema plugin.Schema, config map[string]any) map[string]CredentialRefState {
	out := map[string]CredentialRefState{}
	for _, key := range credentialRefKeys(schema) {
		id, _ := config[key].(string)
		if strings.TrimSpace(id) == "" {
			out[key] = CredentialRefState{State: "not_set"}
			continue
		}
		cred, ok := s.creds.SummaryIfUsable(ctx, userID, id)
		if !ok {
			delete(config, key)
			out[key] = CredentialRefState{State: "set", Readable: false}
			continue
		}
		out[key] = CredentialRefState{State: "set", Readable: true, Summary: &cred}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func resolveFolderColor(color string) (string, error) {
	color = strings.TrimSpace(color)
	if color == "" {
		return defaultFolderColor, nil
	}
	if !folderColors[color] {
		return "", fmt.Errorf("%w: unsupported folder color %q", plugin.ErrInvalidInput, color)
	}
	return color, nil
}

func folderExists(folders []models.ConnectionFolder, id string) bool {
	for _, f := range folders {
		if f.ID == id {
			return true
		}
	}
	return false
}

func hasFolderCycle(parentByID map[string]string) bool {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if id == "" || visited[id] {
			return false
		}
		if visiting[id] {
			return true
		}
		visiting[id] = true
		if visit(parentByID[id]) {
			return true
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for id := range parentByID {
		if visit(id) {
			return true
		}
	}
	return false
}

func nextFolderOrder(folders []models.ConnectionFolder, parentID string) int {
	maxOrder := -1
	for _, f := range folders {
		if f.ParentID != parentID {
			continue
		}
		if f.SortOrder > maxOrder {
			maxOrder = f.SortOrder
		}
	}
	return maxOrder + 1
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

func (s *ConnectionService) checkCredentialRefsForUpdate(ctx context.Context, actorID string, existing models.Connection, schema plugin.Schema, values map[string]any, preserve []string) error {
	preserved := stringSet(preserve)
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
			if field.Credential != nil && len(field.Credential.Protocols) > 0 && !slices.Contains(field.Credential.Protocols, existing.Protocol) {
				return fmt.Errorf("%w: credential field %q is not valid for protocol %q", plugin.ErrInvalidInput, field.Key, existing.Protocol)
			}
			userID := actorID
			if preserved[field.Key] {
				userID = existing.OwnerID
			}
			if err := s.creds.EnsureUsableFor(ctx, userID, id, kinds, existing.Protocol); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ConnectionService) mergePreservedCredentialRefs(existing models.Connection, schema plugin.Schema, in ConnectionInput) (map[string]any, error) {
	out := map[string]any{}
	maps.Copy(out, in.Config)
	preserved := stringSet(in.PreserveCredentials)
	if len(preserved) == 0 {
		return out, nil
	}
	credentialFields := map[string]bool{}
	for _, key := range credentialRefKeys(schema) {
		credentialFields[key] = true
	}
	for key := range preserved {
		if !credentialFields[key] {
			return nil, fmt.Errorf("%w: cannot preserve unknown credential field %q", plugin.ErrInvalidInput, key)
		}
		if id, _ := existing.Config[key].(string); strings.TrimSpace(id) != "" {
			out[key] = id
		}
	}
	return out, nil
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
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

func connectionSchemaContext(protocol, transport string) map[string]any {
	if transport == "" {
		transport = string(plugin.TransportDirect)
	}
	return map[string]any{
		plugin.SchemaContextProtocol:  protocol,
		plugin.SchemaContextTransport: transport,
	}
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
