package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
)

// NewMemory returns a fully in-memory Store for unit tests — no DB, no gorm.
func NewMemory() *Store {
	return &Store{
		Users:                &memUserStore{users: map[string]models.User{}, hashes: map[string]string{}},
		Connections:          &memConnectionStore{m: map[string]models.Connection{}},
		ConnectionFolders:    &memConnectionFolderStore{m: map[string]models.ConnectionFolder{}},
		ConnectionPlacements: &memConnectionPlacementStore{m: map[string]models.ConnectionPlacement{}},
		Credentials:          &memCredentialStore{m: map[string]models.Credential{}},
		Grants:               &memGrantStore{m: map[string]models.Grant{}},
		CredentialGrants:     &memCredentialGrantStore{m: map[string]models.CredentialGrant{}},
		Audit:                &memAuditStore{},
		PluginStorage:        &memPluginStorageStore{m: map[pluginStorageKey]models.PluginStorageItem{}},
		Preferences:          &memPreferenceStore{m: map[string]models.Preference{}},
		Enrollments:          &memEnrollmentStore{m: map[string]models.AgentEnrollment{}},
		Policies:             &memPolicyStore{m: map[string]models.PolicyRule{}},
		Invitations:          &memInvitationStore{m: map[string]models.Invitation{}},
		Recordings:           &memRecordingStore{m: map[string]models.Recording{}},
		ProtocolSettings:     &memProtocolSettingStore{m: map[string]models.ProtocolSetting{}},
		AIProviders:          &memAIProviderStore{m: map[string]models.AIProviderConfig{}},
		AIConversations:      &memAIConversationStore{m: map[string]models.AIConversation{}},
		AIMessages:           &memAIMessageStore{m: map[string][]models.AIMessage{}},
	}
}

type memAIConversationStore struct {
	mu sync.RWMutex
	m  map[string]models.AIConversation
}

func (s *memAIConversationStore) Create(_ context.Context, c *models.AIConversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[c.ID]; ok {
		return models.ErrConflict
	}
	s.m[c.ID] = *c
	return nil
}

func (s *memAIConversationStore) Get(_ context.Context, id string) (models.AIConversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.m[id]
	if !ok {
		return models.AIConversation{}, ErrNotFound
	}
	return c, nil
}

func (s *memAIConversationStore) List(_ context.Context, ownerID, connectionID string) ([]models.AIConversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.AIConversation
	for _, c := range s.m {
		if c.OwnerID != ownerID {
			continue
		}
		if connectionID != "" && c.ConnectionID != connectionID {
			continue
		}
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
	return list, nil
}

func (s *memAIConversationStore) Update(_ context.Context, c *models.AIConversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, ok := s.m[c.ID]
	if !ok {
		return ErrNotFound
	}
	prev.Title = c.Title
	prev.AutoTitled = c.AutoTitled
	prev.ProviderID = c.ProviderID
	prev.Model = c.Model
	prev.Summary = c.Summary
	prev.UpdatedAt = c.UpdatedAt
	s.m[c.ID] = prev
	return nil
}

func (s *memAIConversationStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type memAIMessageStore struct {
	mu sync.RWMutex
	m  map[string][]models.AIMessage
}

func (s *memAIMessageStore) Append(_ context.Context, m *models.AIMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[m.ConversationID] = append(s.m[m.ConversationID], *m)
	return nil
}

func (s *memAIMessageStore) List(_ context.Context, conversationID string) ([]models.AIMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]models.AIMessage(nil), s.m[conversationID]...)
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
}

func (s *memAIMessageStore) Recent(ctx context.Context, conversationID string, limit int) ([]models.AIMessage, error) {
	all, _ := s.List(ctx, conversationID)
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, nil
}

func (s *memAIMessageStore) Range(ctx context.Context, conversationID string, offset, limit int) ([]models.AIMessage, error) {
	all, _ := s.List(ctx, conversationID)
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (s *memAIMessageStore) Count(_ context.Context, conversationID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m[conversationID]), nil
}

func (s *memAIMessageStore) DeleteByConversation(_ context.Context, conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, conversationID)
	return nil
}

type memAIProviderStore struct {
	mu sync.RWMutex
	m  map[string]models.AIProviderConfig
}

func (s *memAIProviderStore) Create(_ context.Context, c *models.AIProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[c.ID]; ok {
		return models.ErrConflict
	}
	s.m[c.ID] = *c
	return nil
}

func (s *memAIProviderStore) Get(_ context.Context, id string) (models.AIProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.m[id]
	if !ok {
		return models.AIProviderConfig{}, ErrNotFound
	}
	return c, nil
}

func (s *memAIProviderStore) ListByOwner(_ context.Context, ownerID string) ([]models.AIProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.AIProviderConfig
	for _, c := range s.m {
		if c.OwnerID == ownerID {
			list = append(list, c)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	return list, nil
}

func (s *memAIProviderStore) Update(_ context.Context, c *models.AIProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, ok := s.m[c.ID]
	if !ok {
		return ErrNotFound
	}
	prev.Kind = c.Kind
	prev.Name = c.Name
	prev.BaseURL = c.BaseURL
	prev.Models = c.Models
	prev.Model = c.Model
	prev.APIKeyCiphertext = c.APIKeyCiphertext
	prev.UpdatedAt = time.Now()
	s.m[c.ID] = prev
	return nil
}

func (s *memAIProviderStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type memUserStore struct {
	mu     sync.RWMutex
	users  map[string]models.User
	hashes map[string]string
}

func (s *memUserStore) Create(_ context.Context, u *models.User, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.users {
		if existing.Username == u.Username {
			return models.ErrConflict
		}
	}
	s.users[u.ID] = *u
	s.hashes[u.ID] = passwordHash
	return nil
}

func (s *memUserStore) GetByID(_ context.Context, id string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return models.User{}, ErrNotFound
	}
	return u, nil
}

func (s *memUserStore) GetByUsername(_ context.Context, username string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			return u, nil
		}
	}
	return models.User{}, ErrNotFound
}

func (s *memUserStore) GetByEmail(_ context.Context, email string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email != "" && u.Email == email {
			return u, nil
		}
	}
	return models.User{}, ErrNotFound
}

func (s *memUserStore) GetPasswordHash(_ context.Context, userID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.users[userID]; !ok {
		return "", ErrNotFound
	}
	return s.hashes[userID], nil
}

func (s *memUserStore) SetPasswordHash(_ context.Context, userID, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userID]; !ok {
		return ErrNotFound
	}
	s.hashes[userID] = hash
	u := s.users[userID]
	u.SessionVersion++
	s.users[userID] = u
	return nil
}

func (s *memUserStore) List(_ context.Context) ([]models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}

func (s *memUserStore) Update(_ context.Context, u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; !ok {
		return ErrNotFound
	}
	s.users[u.ID] = *u
	return nil
}

func (s *memUserStore) SetTwoFactor(_ context.Context, userID string, secret []byte, enabled bool, recoveryHashes []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.TOTPSecret = secret
	u.TOTPEnabled = enabled
	u.RecoveryCodeHashes = recoveryHashes
	s.users[userID] = u
	return nil
}

func (s *memUserStore) SetMFARemindedAt(_ context.Context, userID string, at *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.MFARemindedAt = at
	s.users[userID] = u
	return nil
}

func (s *memUserStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, id)
	delete(s.hashes, id)
	return nil
}

func (s *memUserStore) Count(_ context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.users)), nil
}

type memConnectionStore struct {
	mu sync.RWMutex
	m  map[string]models.Connection
}

func (s *memConnectionStore) Create(_ context.Context, c *models.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[c.ID]; ok {
		return models.ErrConflict
	}
	s.m[c.ID] = *c
	return nil
}

func (s *memConnectionStore) Get(_ context.Context, id string) (models.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.m[id]
	if !ok {
		return models.Connection{}, ErrNotFound
	}
	return c, nil
}

func (s *memConnectionStore) ListByOwner(_ context.Context, ownerID string) ([]models.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Connection
	for _, c := range s.m {
		if c.OwnerID == ownerID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *memConnectionStore) List(_ context.Context) ([]models.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Connection, 0, len(s.m))
	for _, c := range s.m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *memConnectionStore) Update(_ context.Context, c *models.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[c.ID]; !ok {
		return ErrNotFound
	}
	s.m[c.ID] = *c
	return nil
}

func (s *memConnectionStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type memConnectionFolderStore struct {
	mu sync.RWMutex
	m  map[string]models.ConnectionFolder
}

func (s *memConnectionFolderStore) Create(_ context.Context, f *models.ConnectionFolder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[f.ID]; ok {
		return models.ErrConflict
	}
	s.m[f.ID] = *f
	return nil
}

func (s *memConnectionFolderStore) Get(_ context.Context, id string) (models.ConnectionFolder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.m[id]
	if !ok {
		return models.ConnectionFolder{}, ErrNotFound
	}
	return f, nil
}

func (s *memConnectionFolderStore) ListByUser(_ context.Context, userID string) ([]models.ConnectionFolder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.ConnectionFolder
	for _, f := range s.m {
		if f.UserID == userID {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].Name < out[j].Name
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out, nil
}

func (s *memConnectionFolderStore) Update(_ context.Context, f *models.ConnectionFolder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[f.ID]; !ok {
		return ErrNotFound
	}
	s.m[f.ID] = *f
	return nil
}

func (s *memConnectionFolderStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type memConnectionPlacementStore struct {
	mu sync.RWMutex
	m  map[string]models.ConnectionPlacement
}

func placementKey(userID, connectionID string) string {
	return userID + "\x00" + connectionID
}

func (s *memConnectionPlacementStore) ListByUser(_ context.Context, userID string) ([]models.ConnectionPlacement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.ConnectionPlacement
	for _, p := range s.m {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *memConnectionPlacementStore) Set(_ context.Context, p *models.ConnectionPlacement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[placementKey(p.UserID, p.ConnectionID)] = *p
	return nil
}

func (s *memConnectionPlacementStore) Delete(_ context.Context, userID, connectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, placementKey(userID, connectionID))
	return nil
}

func (s *memConnectionPlacementStore) DeleteByConnection(_ context.Context, connectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, p := range s.m {
		if p.ConnectionID == connectionID {
			delete(s.m, key)
		}
	}
	return nil
}

func (s *memConnectionPlacementStore) ClearFolder(ctx context.Context, userID, folderID string) error {
	return s.MoveFolder(ctx, userID, folderID, "")
}

func (s *memConnectionPlacementStore) MoveFolder(_ context.Context, userID, folderID, targetFolderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, p := range s.m {
		if p.UserID == userID && p.FolderID == folderID {
			p.FolderID = targetFolderID
			p.UpdatedAt = time.Now()
			s.m[key] = p
		}
	}
	return nil
}

type memCredentialStore struct {
	mu sync.RWMutex
	m  map[string]models.Credential
}

func (s *memCredentialStore) Create(_ context.Context, c *models.Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[c.ID]; ok {
		return models.ErrConflict
	}
	s.m[c.ID] = *c
	return nil
}

func (s *memCredentialStore) Get(_ context.Context, id string) (models.Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.m[id]
	if !ok {
		return models.Credential{}, ErrNotFound
	}
	return c, nil
}

func (s *memCredentialStore) ListByOwner(_ context.Context, ownerID string) ([]models.Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Credential
	for _, c := range s.m {
		if c.OwnerID == ownerID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *memCredentialStore) Update(_ context.Context, c *models.Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[c.ID]; !ok {
		return ErrNotFound
	}
	s.m[c.ID] = *c
	return nil
}

func (s *memCredentialStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type memGrantStore struct {
	mu sync.RWMutex
	m  map[string]models.Grant
}

func (s *memGrantStore) Create(_ context.Context, g *models.Grant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.m {
		if existing.ConnectionID == g.ConnectionID && existing.SubjectID == g.SubjectID {
			return models.ErrConflict
		}
	}
	s.m[g.ID] = *g
	return nil
}

func (s *memGrantStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func (s *memGrantStore) Get(_ context.Context, connectionID, subjectID string) (models.Grant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.m {
		if g.ConnectionID == connectionID && g.SubjectID == subjectID {
			return g, nil
		}
	}
	return models.Grant{}, ErrNotFound
}

func (s *memGrantStore) ListByConnection(_ context.Context, connectionID string) ([]models.Grant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Grant
	for _, g := range s.m {
		if g.ConnectionID == connectionID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (s *memGrantStore) ListBySubject(_ context.Context, subjectID string) ([]models.Grant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Grant
	for _, g := range s.m {
		if g.SubjectID == subjectID {
			out = append(out, g)
		}
	}
	return out, nil
}

type memCredentialGrantStore struct {
	mu sync.RWMutex
	m  map[string]models.CredentialGrant
}

func (s *memCredentialGrantStore) Create(_ context.Context, g *models.CredentialGrant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.m {
		if existing.CredentialID == g.CredentialID && existing.SubjectID == g.SubjectID {
			return models.ErrConflict
		}
	}
	s.m[g.ID] = *g
	return nil
}

func (s *memCredentialGrantStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func (s *memCredentialGrantStore) Has(_ context.Context, credentialID, subjectID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.m {
		if g.CredentialID == credentialID && g.SubjectID == subjectID {
			return true, nil
		}
	}
	return false, nil
}

func (s *memCredentialGrantStore) ListByCredential(_ context.Context, credentialID string) ([]models.CredentialGrant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.CredentialGrant
	for _, g := range s.m {
		if g.CredentialID == credentialID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (s *memCredentialGrantStore) ListBySubject(_ context.Context, subjectID string) ([]models.CredentialGrant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.CredentialGrant
	for _, g := range s.m {
		if g.SubjectID == subjectID {
			out = append(out, g)
		}
	}
	return out, nil
}

type memAuditStore struct {
	mu      sync.RWMutex
	entries []models.AuditEntry
}

func (s *memAuditStore) Append(_ context.Context, e *models.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, *e)
	return nil
}

func (s *memAuditStore) matches(e models.AuditEntry, f AuditFilter) bool {
	if f.UserID != "" && e.UserID != f.UserID {
		return false
	}
	if f.ConnectionID != "" && e.ConnectionID != f.ConnectionID {
		return false
	}
	return true
}

func (s *memAuditStore) List(_ context.Context, f AuditFilter) ([]models.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.AuditEntry
	skipped := 0
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if !s.matches(e, f) {
			continue
		}
		if f.Offset > 0 && skipped < f.Offset {
			skipped++
			continue
		}
		out = append(out, e)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out, nil
}

func (s *memAuditStore) Count(_ context.Context, f AuditFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var n int64
	for _, e := range s.entries {
		if s.matches(e, f) {
			n++
		}
	}
	return n, nil
}

func (s *memAuditStore) DeleteBefore(_ context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []models.AuditEntry
	var removed int64

	for _, e := range s.entries {
		if e.Time.Before(before) {
			removed++
			continue
		}

		kept = append(kept, e)
	}

	s.entries = kept

	return removed, nil
}

type pluginStorageKey struct {
	collection   string
	plugin       string
	connectionID string
	ownerID      string
	key          string
}

type memPluginStorageStore struct {
	mu sync.RWMutex
	m  map[pluginStorageKey]models.PluginStorageItem
}

type memPolicyStore struct {
	mu sync.RWMutex
	m  map[string]models.PolicyRule
}

func (s *memPolicyStore) Create(_ context.Context, p *models.PolicyRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.m {
		if existing.Role == p.Role && existing.Permission == p.Permission && existing.Risk == p.Risk {
			return models.ErrConflict
		}
	}
	s.m[p.ID] = *p
	return nil
}

func (s *memPolicyStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func (s *memPolicyStore) List(_ context.Context) ([]models.PolicyRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.PolicyRule, 0, len(s.m))
	for _, p := range s.m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *memPluginStorageStore) Get(_ context.Context, f PluginStorageFilter) (models.PluginStorageItem, error) {
	if err := validatePluginStorageFilter(f, pluginStorageFilterRead); err != nil {
		return models.PluginStorageItem{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found *models.PluginStorageItem
	for _, item := range s.m {
		if pluginStorageMatches(item, f) {
			if pluginStorageKeyNeedsUniqueConnection(f) {
				if found != nil {
					return models.PluginStorageItem{}, models.ErrConflict
				}
				cp := item
				found = &cp
				continue
			}
			return item, nil
		}
	}
	if found != nil {
		return *found, nil
	}
	return models.PluginStorageItem{}, ErrNotFound
}

func (s *memPluginStorageStore) Put(_ context.Context, item *models.PluginStorageItem) error {
	if err := validatePluginStoragePut(item); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[pluginStorageKeyOf(*item)] = *item
	return nil
}

func (s *memPluginStorageStore) Delete(_ context.Context, f PluginStorageFilter) error {
	if err := validatePluginStorageFilter(f, pluginStorageFilterListOrDelete); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if pluginStorageKeyNeedsUniqueConnection(f) {
		matches := 0
		for _, item := range s.m {
			if pluginStorageMatches(item, f) {
				matches++
				if matches > 1 {
					return models.ErrConflict
				}
			}
		}
		if matches == 0 {
			return ErrNotFound
		}
	}
	deleted := false
	for key, item := range s.m {
		if pluginStorageMatches(item, f) {
			delete(s.m, key)
			deleted = true
		}
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func (s *memPluginStorageStore) List(_ context.Context, f PluginStorageFilter) ([]models.PluginStorageItem, error) {
	if err := validatePluginStorageFilter(f, pluginStorageFilterListOrDelete); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.PluginStorageItem
	for _, item := range s.m {
		if pluginStorageMatches(item, f) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Collection != out[j].Collection {
			return out[i].Collection < out[j].Collection
		}
		if out[i].Plugin != out[j].Plugin {
			return out[i].Plugin < out[j].Plugin
		}
		if out[i].ConnectionID != out[j].ConnectionID {
			return out[i].ConnectionID < out[j].ConnectionID
		}
		if out[i].OwnerID != out[j].OwnerID {
			return out[i].OwnerID < out[j].OwnerID
		}
		return out[i].ItemKey < out[j].ItemKey
	})
	return out, nil
}

func pluginStorageKeyOf(item models.PluginStorageItem) pluginStorageKey {
	return pluginStorageKey{
		collection:   item.Collection,
		plugin:       item.Plugin,
		connectionID: item.ConnectionID,
		ownerID:      item.OwnerID,
		key:          item.ItemKey,
	}
}

func pluginStorageMatches(item models.PluginStorageItem, f PluginStorageFilter) bool {
	if f.Collection != "" && item.Collection != f.Collection {
		return false
	}
	if f.Plugin != "" && item.Plugin != f.Plugin {
		return false
	}
	if f.ConnectionID != "" && item.ConnectionID != f.ConnectionID {
		return false
	}
	if f.OwnerID != "" && item.OwnerID != f.OwnerID {
		return false
	}
	if f.Key != "" && item.ItemKey != f.Key {
		return false
	}
	return true
}

type memPreferenceStore struct {
	mu sync.RWMutex
	m  map[string]models.Preference
}

func prefKey(userID, key string) string { return userID + "\x00" + key }

func (s *memPreferenceStore) Get(_ context.Context, userID, key string) (models.Preference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.m[prefKey(userID, key)]
	if !ok {
		return models.Preference{}, ErrNotFound
	}
	return p, nil
}

func (s *memPreferenceStore) Set(_ context.Context, p *models.Preference) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *p
	cp.UpdatedAt = time.Now()
	s.m[prefKey(p.UserID, p.Key)] = cp
	return nil
}

func (s *memPreferenceStore) Delete(_ context.Context, userID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, prefKey(userID, key))
	return nil
}

type memProtocolSettingStore struct {
	mu sync.RWMutex
	m  map[string]models.ProtocolSetting
}

func (s *memProtocolSettingStore) List(_ context.Context) ([]models.ProtocolSetting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.ProtocolSetting, 0, len(s.m))
	for _, p := range s.m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Protocol < out[j].Protocol })
	return out, nil
}

func (s *memProtocolSettingStore) Set(_ context.Context, p *models.ProtocolSetting) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *p
	cp.UpdatedAt = time.Now()
	s.m[p.Protocol] = cp
	return nil
}

type memInvitationStore struct {
	mu sync.RWMutex
	m  map[string]models.Invitation
}

func (s *memInvitationStore) Create(_ context.Context, i *models.Invitation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.m {
		if existing.TokenHash == i.TokenHash {
			return models.ErrConflict
		}
	}
	s.m[i.ID] = *i
	return nil
}

func (s *memInvitationStore) Get(_ context.Context, id string) (models.Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.m[id]
	if !ok {
		return models.Invitation{}, ErrNotFound
	}
	return i, nil
}

func (s *memInvitationStore) GetByTokenHash(_ context.Context, tokenHash string) (models.Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, i := range s.m {
		if i.TokenHash == tokenHash {
			return i, nil
		}
	}
	return models.Invitation{}, ErrNotFound
}

func (s *memInvitationStore) List(_ context.Context) ([]models.Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Invitation, 0, len(s.m))
	for _, i := range s.m {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].CreatedAt.After(out[b].CreatedAt) })
	return out, nil
}

func (s *memInvitationStore) Update(_ context.Context, i *models.Invitation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[i.ID]; !ok {
		return ErrNotFound
	}
	s.m[i.ID] = *i
	return nil
}

func (s *memInvitationStore) Consume(_ context.Context, id string, acceptedAt time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.m[id]
	if !ok || i.Status != models.InvitePending || !acceptedAt.Before(i.ExpiresAt) {
		return false, nil
	}
	i.Status = models.InviteAccepted
	i.AcceptedAt = acceptedAt
	s.m[id] = i
	return true, nil
}

func (s *memInvitationStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type memEnrollmentStore struct {
	mu sync.RWMutex
	m  map[string]models.AgentEnrollment
}

func (s *memEnrollmentStore) Create(_ context.Context, e *models.AgentEnrollment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.m {
		if existing.TokenHash == e.TokenHash {
			return models.ErrConflict
		}
	}
	s.m[e.ID] = *e
	return nil
}

func (s *memEnrollmentStore) Get(_ context.Context, id string) (models.AgentEnrollment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.m[id]
	if !ok {
		return models.AgentEnrollment{}, ErrNotFound
	}
	return e, nil
}

func (s *memEnrollmentStore) GetByTokenHash(_ context.Context, tokenHash string) (models.AgentEnrollment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.m {
		if e.TokenHash == tokenHash {
			return e, nil
		}
	}
	return models.AgentEnrollment{}, ErrNotFound
}

func (s *memEnrollmentStore) ListByConnection(_ context.Context, connectionID string) ([]models.AgentEnrollment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.AgentEnrollment
	for _, e := range s.m {
		if e.ConnectionID == connectionID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *memEnrollmentStore) UpdateStatus(_ context.Context, id string, status models.AgentEnrollmentStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = status
	e.UpdatedAt = time.Now()
	s.m[id] = e
	return nil
}

func (s *memEnrollmentStore) UpdateToken(_ context.Context, id, tokenHash string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[id]
	if !ok {
		return ErrNotFound
	}
	e.TokenHash = tokenHash
	e.ExpiresAt = expiresAt
	e.UpdatedAt = time.Now()
	s.m[id] = e
	return nil
}

func (s *memEnrollmentStore) Consume(_ context.Context, id string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[id]
	if !ok {
		return false, nil
	}
	switch e.Status {
	case models.EnrollmentPending:
		if !now.Before(e.ExpiresAt) {
			return false, nil
		}
	case models.EnrollmentOffline, models.EnrollmentOnline:
	default:
		return false, nil
	}
	e.Status = models.EnrollmentOnline
	e.UpdatedAt = now
	s.m[id] = e
	return true, nil
}

type memRecordingStore struct {
	mu sync.RWMutex
	m  map[string]models.Recording
}

func (s *memRecordingStore) Create(_ context.Context, r *models.Recording) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[r.ID]; ok {
		return models.ErrConflict
	}
	s.m[r.ID] = *r
	return nil
}

func (s *memRecordingStore) Get(_ context.Context, id string) (models.Recording, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.m[id]
	if !ok {
		return models.Recording{}, ErrNotFound
	}
	return r, nil
}

func (s *memRecordingStore) Update(_ context.Context, r *models.Recording) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, ok := s.m[r.ID]
	if !ok {
		return ErrNotFound
	}
	prev.Status = r.Status
	prev.Title = r.Title
	prev.EndedAt = r.EndedAt
	prev.DurationMS = r.DurationMS
	prev.Size = r.Size
	prev.Checksum = r.Checksum
	prev.StorageKey = r.StorageKey
	prev.Error = r.Error
	prev.ExpiresAt = r.ExpiresAt
	prev.UpdatedAt = time.Now()
	s.m[r.ID] = prev
	return nil
}

func (s *memRecordingStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func (s *memRecordingStore) List(_ context.Context, f RecordingFilter) ([]models.Recording, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Recording
	for _, r := range s.m {
		if !recordingMatches(r, f) {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func recordingMatches(r models.Recording, f RecordingFilter) bool {
	switch {
	case f.UserID != "" && r.UserID != f.UserID:
		return false
	case f.ConnectionID != "" && r.ConnectionID != f.ConnectionID:
		return false
	case f.Protocol != "" && r.Protocol != f.Protocol:
		return false
	case f.Class != "" && r.Class != f.Class:
		return false
	case f.Format != "" && r.Format != f.Format:
		return false
	case f.Status != "" && string(r.Status) != f.Status:
		return false
	case !f.Since.IsZero() && r.StartedAt.Before(f.Since):
		return false
	case !f.Until.IsZero() && r.StartedAt.After(f.Until):
		return false
	case !f.ExpiredBefore.IsZero() && (r.ExpiresAt == nil || r.ExpiresAt.After(f.ExpiredBefore)):
		return false
	}
	return true
}
