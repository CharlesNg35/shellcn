package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/store"
)

// storeFactory builds a fresh Store for one test run.
type storeFactory struct {
	name string
	open func(t *testing.T) *store.Store
}

// factories returns every backend the suite runs against. SQLite + the in-memory
// fake are the per-PR gate; Postgres/MySQL run only when their DSN env is set
// (nightly / M1 hardening).
func factories() []storeFactory {
	fs := []storeFactory{
		{name: "memory", open: func(_ *testing.T) *store.Store { return store.NewMemory() }},
		{name: "sqlite", open: func(t *testing.T) *store.Store {
			dsn := filepath.Join(t.TempDir(), "test.db")
			s, err := store.Open(store.Config{Driver: store.DriverSQLite, DSN: dsn})
			if err != nil {
				t.Fatalf("open sqlite: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}},
	}
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		fs = append(fs, storeFactory{name: "postgres", open: func(t *testing.T) *store.Store {
			s, err := store.Open(store.Config{Driver: store.DriverPostgres, DSN: dsn})
			if err != nil {
				t.Fatalf("open postgres: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}})
	}
	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		fs = append(fs, storeFactory{name: "mysql", open: func(t *testing.T) *store.Store {
			s, err := store.Open(store.Config{Driver: store.DriverMySQL, DSN: dsn})
			if err != nil {
				t.Fatalf("open mysql: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}})
	}
	return fs
}

func TestStoreSuite(t *testing.T) {
	for _, f := range factories() {
		t.Run(f.name, func(t *testing.T) {
			t.Run("users", func(t *testing.T) { testUsers(t, f.open(t)) })
			t.Run("connections", func(t *testing.T) { testConnections(t, f.open(t)) })
			t.Run("credentials", func(t *testing.T) { testCredentials(t, f.open(t)) })
			t.Run("grants", func(t *testing.T) { testGrants(t, f.open(t)) })
			t.Run("credentialReference", func(t *testing.T) { testCredentialReference(t, f.open(t)) })
			t.Run("audit", func(t *testing.T) { testAudit(t, f.open(t)) })
			t.Run("policies", func(t *testing.T) { testPolicies(t, f.open(t)) })
		})
	}
}

func testUsers(t *testing.T, s *store.Store) {
	ctx := context.Background()
	u := &models.User{ID: "u1", Username: "alice", Email: "a@x", Roles: []models.Role{models.RoleAdmin}}
	if err := s.Users.Create(ctx, u, "hash1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if n, _ := s.Users.Count(ctx); n != 1 {
		t.Errorf("count: want 1, got %d", n)
	}

	got, err := s.Users.GetByUsername(ctx, "alice")
	if err != nil || got.ID != "u1" || !got.HasRole(models.RoleAdmin) {
		t.Fatalf("get by username: %+v err=%v", got, err)
	}
	if h, _ := s.Users.GetPasswordHash(ctx, "u1"); h != "hash1" {
		t.Errorf("password hash: want hash1, got %q", h)
	}

	// Duplicate username rejected.
	if err := s.Users.Create(ctx, &models.User{ID: "u2", Username: "alice"}, "x"); err == nil {
		t.Error("duplicate username should be rejected")
	}

	got.Email = "alice@new"
	got.Roles = []models.Role{models.RoleOperator}
	if err := s.Users.Update(ctx, &got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, _ := s.Users.GetByID(ctx, "u1")
	if reloaded.Email != "alice@new" || !reloaded.HasRole(models.RoleOperator) {
		t.Errorf("update not persisted: %+v", reloaded)
	}

	if err := s.Users.SetPasswordHash(ctx, "u1", "hash2"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if h, _ := s.Users.GetPasswordHash(ctx, "u1"); h != "hash2" {
		t.Errorf("password not rotated: %q", h)
	}

	if err := s.Users.Delete(ctx, "u1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Users.GetByID(ctx, "u1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("get deleted: want ErrNotFound, got %v", err)
	}
}

func testConnections(t *testing.T, s *store.Store) {
	ctx := context.Background()
	c := &models.Connection{
		ID: "c1", Name: "prod-web", Protocol: "ssh", OwnerID: "u1", Transport: "direct",
		Config:  map[string]any{"host": "10.0.0.1", "port": float64(22)},
		Secrets: map[string][]byte{"password": []byte("ciphertext")},
	}
	if err := s.Connections.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Connections.Get(ctx, "c1")
	if err != nil || got.Config["host"] != "10.0.0.1" {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	if string(got.Secrets["password"]) != "ciphertext" {
		t.Errorf("secrets ciphertext not round-tripped: %v", got.Secrets)
	}

	list, _ := s.Connections.ListByOwner(ctx, "u1")
	if len(list) != 1 {
		t.Errorf("list by owner: want 1, got %d", len(list))
	}
	if other, _ := s.Connections.ListByOwner(ctx, "nobody"); len(other) != 0 {
		t.Errorf("list by other owner: want 0, got %d", len(other))
	}

	got.Name = "prod-web-renamed"
	if err := s.Connections.Update(ctx, &got); err != nil {
		t.Fatalf("update: %v", err)
	}
	if reloaded, _ := s.Connections.Get(ctx, "c1"); reloaded.Name != "prod-web-renamed" {
		t.Errorf("update not persisted: %q", reloaded.Name)
	}

	if err := s.Connections.Delete(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Connections.Get(ctx, "c1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("get deleted: want ErrNotFound, got %v", err)
	}
}

func testCredentials(t *testing.T, s *store.Store) {
	ctx := context.Background()
	cr := &models.Credential{
		ID: "cr1", Name: "ops key", Kind: "ssh_private_key", OwnerID: "u1",
		Username: "ops", Protocols: []string{"ssh"}, EncryptedSecret: []byte("enc-key"),
	}
	if err := s.Credentials.Create(ctx, cr); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Credentials.Get(ctx, "cr1")
	if err != nil || string(got.EncryptedSecret) != "enc-key" {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	if sum := got.Summary(); len(sum.Protocols) != 1 || sum.Protocols[0] != "ssh" {
		t.Errorf("summary protocols: %+v", sum)
	}
	list, _ := s.Credentials.ListByOwner(ctx, "u1")
	if len(list) != 1 {
		t.Errorf("list: want 1, got %d", len(list))
	}
	if err := s.Credentials.Delete(ctx, "cr1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func testGrants(t *testing.T, s *store.Store) {
	ctx := context.Background()
	g := &models.Grant{ID: "g1", ConnectionID: "c1", SubjectID: "u2", Access: models.AccessUse}
	if err := s.Grants.Create(ctx, g); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Duplicate (same connection + subject) rejected by the unique index.
	if err := s.Grants.Create(ctx, &models.Grant{ID: "g2", ConnectionID: "c1", SubjectID: "u2", Access: models.AccessUse}); err == nil {
		t.Error("duplicate grant should be rejected")
	}
	got, err := s.Grants.Get(ctx, "c1", "u2")
	if err != nil || got.Access != models.AccessUse {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	if byConn, _ := s.Grants.ListByConnection(ctx, "c1"); len(byConn) != 1 {
		t.Errorf("by connection: want 1, got %d", len(byConn))
	}
	if bySub, _ := s.Grants.ListBySubject(ctx, "u2"); len(bySub) != 1 {
		t.Errorf("by subject: want 1, got %d", len(bySub))
	}
	if err := s.Grants.Delete(ctx, "g1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// testCredentialReference proves a connection can point at a reusable credential
// (use-grant present) without duplicating the secret material.
func testCredentialReference(t *testing.T, s *store.Store) {
	ctx := context.Background()
	cr := &models.Credential{ID: "cr-shared", Name: "shared", Kind: "ssh_password", OwnerID: "owner", EncryptedSecret: []byte("enc")}
	if err := s.Credentials.Create(ctx, cr); err != nil {
		t.Fatalf("create credential: %v", err)
	}
	conn := &models.Connection{
		ID: "c-ref", Name: "ref", Protocol: "ssh", OwnerID: "other", Transport: "direct",
		Config: map[string]any{"credential_id": "cr-shared"},
	}
	if err := s.Connections.Create(ctx, conn); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	// The connection references the credential by id only — no secret copied.
	got, _ := s.Connections.Get(ctx, "c-ref")
	if got.Config["credential_id"] != "cr-shared" {
		t.Fatalf("connection does not reference credential: %+v", got.Config)
	}
	if len(got.Secrets) != 0 {
		t.Errorf("referencing connection should carry no inline secret, got %v", got.Secrets)
	}

	// A use-grant lets "other" connect through it; readback of the value is never
	// offered by the store API (only Get returns ciphertext to the service layer).
	if err := s.CredentialGrants.Create(ctx, &models.CredentialGrant{ID: "cg1", CredentialID: "cr-shared", SubjectID: "other", Access: models.AccessUse}); err != nil {
		t.Fatalf("create credential grant: %v", err)
	}
	has, _ := s.CredentialGrants.Has(ctx, "cr-shared", "other")
	if !has {
		t.Error("expected credential use-grant for subject")
	}
}

func testAudit(t *testing.T, s *store.Store) {
	ctx := context.Background()
	for i := range 3 {
		e := &models.AuditEntry{
			ID: "a" + string(rune('0'+i)), Time: time.Now().Add(time.Duration(i) * time.Second),
			UserID: "u1", Username: "alice", Event: "vm.start", ConnectionID: "c1",
			RouteID: "proxmox.vm.start", Risk: "write", Result: models.AuditAllowed,
			Params: map[string]string{"vmid": "101"},
		}
		if err := s.Audit.Append(ctx, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	all, err := s.Audit.List(ctx, store.AuditFilter{ConnectionID: "c1"})
	if err != nil || len(all) != 3 {
		t.Fatalf("list: got %d err=%v", len(all), err)
	}
	// Newest first.
	if !all[0].Time.After(all[1].Time) {
		t.Errorf("audit not ordered newest-first")
	}
	limited, _ := s.Audit.List(ctx, store.AuditFilter{UserID: "u1", Limit: 2})
	if len(limited) != 2 {
		t.Errorf("limit: want 2, got %d", len(limited))
	}
}

func testPolicies(t *testing.T, s *store.Store) {
	ctx := context.Background()
	rule := &models.PolicyRule{
		ID: "p1", Role: "auditor", Permission: "audit.read", Risk: "safe", CreatedAt: time.Now(),
	}
	if err := s.Policies.Create(ctx, rule); err != nil {
		t.Fatalf("create: %v", err)
	}
	list, err := s.Policies.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Role != "auditor" || list[0].Permission != "audit.read" || list[0].Risk != "safe" {
		t.Fatalf("unexpected policies: %+v", list)
	}
	if err := s.Policies.Delete(ctx, "p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, _ = s.Policies.List(ctx)
	if len(list) != 0 {
		t.Fatalf("delete did not remove policy: %+v", list)
	}
}
