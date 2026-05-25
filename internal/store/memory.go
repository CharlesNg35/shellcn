package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/charlesng/shellcn/internal/models"
)

// NewMemory returns a fully in-memory Store for unit tests — no DB, no gorm.
func NewMemory() *Store {
	return &Store{
		Users:            &memUserStore{users: map[string]models.User{}, hashes: map[string]string{}},
		Connections:      &memConnectionStore{m: map[string]models.Connection{}},
		Credentials:      &memCredentialStore{m: map[string]models.Credential{}},
		Grants:           &memGrantStore{m: map[string]models.Grant{}},
		CredentialGrants: &memCredentialGrantStore{m: map[string]models.CredentialGrant{}},
		Audit:            &memAuditStore{},
		Snippets:         &memSnippetStore{m: map[string]models.Snippet{}},
		Preferences:      &memPreferenceStore{m: map[string]models.Preference{}},
		Enrollments:      &memEnrollmentStore{m: map[string]models.AgentEnrollment{}},
		Policies:         &memPolicyStore{m: map[string]models.PolicyRule{}},
		Invitations:      &memInvitationStore{m: map[string]models.Invitation{}},
	}
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

func (s *memAuditStore) List(_ context.Context, f AuditFilter) ([]models.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.AuditEntry
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if f.UserID != "" && e.UserID != f.UserID {
			continue
		}
		if f.ConnectionID != "" && e.ConnectionID != f.ConnectionID {
			continue
		}
		out = append(out, e)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out, nil
}

type memSnippetStore struct {
	mu sync.RWMutex
	m  map[string]models.Snippet
}

type memPolicyStore struct {
	mu sync.RWMutex
	m  map[string]models.PolicyRule
}

func (s *memPolicyStore) Create(_ context.Context, p *models.PolicyRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *memSnippetStore) Create(_ context.Context, sn *models.Snippet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[sn.ID] = *sn
	return nil
}

func (s *memSnippetStore) Get(_ context.Context, id string) (models.Snippet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sn, ok := s.m[id]
	if !ok {
		return models.Snippet{}, ErrNotFound
	}
	return sn, nil
}

func (s *memSnippetStore) ListByOwner(_ context.Context, ownerID, protocol string) ([]models.Snippet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Snippet
	for _, sn := range s.m {
		if sn.OwnerID == ownerID && (protocol == "" || sn.Protocol == protocol) {
			out = append(out, sn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *memSnippetStore) Update(_ context.Context, sn *models.Snippet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[sn.ID]; !ok {
		return ErrNotFound
	}
	s.m[sn.ID] = *sn
	return nil
}

func (s *memSnippetStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
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

type memInvitationStore struct {
	mu sync.RWMutex
	m  map[string]models.Invitation
}

func (s *memInvitationStore) Create(_ context.Context, i *models.Invitation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
