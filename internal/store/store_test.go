package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

// storeFactory builds a fresh Store for one test run.
type storeFactory struct {
	name string
	open func(t *testing.T) *store.Store
}

// factories returns every configured backend for the store suite.
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
			t.Run("recordings", func(t *testing.T) { testRecordings(t, f.open(t)) })
			t.Run("pluginStorage", func(t *testing.T) { testPluginStorage(t, f.open(t)) })
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
	reloaded, _ = s.Users.GetByID(ctx, "u1")
	if reloaded.SessionVersion != 1 {
		t.Errorf("session version after password rotation: want 1, got %d", reloaded.SessionVersion)
	}

	// Two-factor state round-trips: encrypted secret bytes, the enabled flag, and
	// the JSON-serialized recovery code hashes.
	secret := []byte{0x01, 0x02, 0x03, 0xff}
	hashes := []string{"hash-a", "hash-b"}
	if err := s.Users.SetTwoFactor(ctx, "u1", secret, true, hashes); err != nil {
		t.Fatalf("set two-factor: %v", err)
	}
	reloaded, _ = s.Users.GetByID(ctx, "u1")
	if !reloaded.TOTPEnabled || !bytes.Equal(reloaded.TOTPSecret, secret) ||
		!slices.Equal(reloaded.RecoveryCodeHashes, hashes) {
		t.Fatalf("two-factor not persisted: enabled=%v secret=%v hashes=%v",
			reloaded.TOTPEnabled, reloaded.TOTPSecret, reloaded.RecoveryCodeHashes)
	}

	when := time.Now().UTC().Truncate(time.Second)
	if err := s.Users.SetMFARemindedAt(ctx, "u1", &when); err != nil {
		t.Fatalf("set reminded: %v", err)
	}
	reloaded, _ = s.Users.GetByID(ctx, "u1")
	if reloaded.MFARemindedAt == nil || !reloaded.MFARemindedAt.UTC().Equal(when) {
		t.Errorf("reminded-at not persisted: %v", reloaded.MFARemindedAt)
	}

	// Disabling clears the secret and recovery codes.
	if err := s.Users.SetTwoFactor(ctx, "u1", nil, false, nil); err != nil {
		t.Fatalf("clear two-factor: %v", err)
	}
	reloaded, _ = s.Users.GetByID(ctx, "u1")
	if reloaded.TOTPEnabled || len(reloaded.TOTPSecret) != 0 || len(reloaded.RecoveryCodeHashes) != 0 {
		t.Errorf("two-factor not cleared: %+v", reloaded)
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

	folder := &models.ConnectionFolder{ID: "f1", UserID: "u1", Name: "Production", Color: "blue", SortOrder: 1}
	if err := s.ConnectionFolders.Create(ctx, folder); err != nil {
		t.Fatalf("folder create: %v", err)
	}
	childFolder := &models.ConnectionFolder{ID: "f2", UserID: "u1", ParentID: "f1", Name: "Databases", Color: "teal", SortOrder: 0}
	if err := s.ConnectionFolders.Create(ctx, childFolder); err != nil {
		t.Fatalf("child folder create: %v", err)
	}
	if err := s.ConnectionPlacements.Set(ctx, &models.ConnectionPlacement{
		UserID: "u1", ConnectionID: "c1", FolderID: "f1", SortOrder: 3,
	}); err != nil {
		t.Fatalf("placement set: %v", err)
	}
	folders, _ := s.ConnectionFolders.ListByUser(ctx, "u1")
	if len(folders) != 2 || !slices.ContainsFunc(folders, func(f models.ConnectionFolder) bool {
		return f.ID == "f2" && f.ParentID == "f1"
	}) {
		t.Fatalf("folders not listed: %+v", folders)
	}
	placements, _ := s.ConnectionPlacements.ListByUser(ctx, "u1")
	if len(placements) != 1 || placements[0].FolderID != "f1" || placements[0].SortOrder != 3 {
		t.Fatalf("placement not listed: %+v", placements)
	}
	if err := s.ConnectionPlacements.ClearFolder(ctx, "u1", "f1"); err != nil {
		t.Fatalf("clear folder: %v", err)
	}
	placements, _ = s.ConnectionPlacements.ListByUser(ctx, "u1")
	if placements[0].FolderID != "" {
		t.Fatalf("clear folder did not move placement to root: %+v", placements)
	}
	if err := s.ConnectionPlacements.Set(ctx, &models.ConnectionPlacement{
		UserID: "u1", ConnectionID: "c1", FolderID: "f1", SortOrder: 3,
	}); err != nil {
		t.Fatalf("placement set for move: %v", err)
	}
	if err := s.ConnectionPlacements.MoveFolder(ctx, "u1", "f1", "f2"); err != nil {
		t.Fatalf("move folder placements: %v", err)
	}
	placements, _ = s.ConnectionPlacements.ListByUser(ctx, "u1")
	if placements[0].FolderID != "f2" {
		t.Fatalf("move folder did not update placement: %+v", placements)
	}

	if err := s.Connections.Delete(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Connections.Get(ctx, "c1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("get deleted: want ErrNotFound, got %v", err)
	}
}

func testPluginStorage(t *testing.T, s *store.Store) {
	ctx := context.Background()
	scope := store.PluginStorageFilter{
		Namespace:    "snippets",
		Plugin:       "ssh",
		OwnerID:      "u1",
		ConnectionID: "c1",
	}
	item := &models.PluginStorageItem{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
		ItemKey:      "prod/restart",
		Value:        []byte("systemctl restart app"),
		ContentType:  "text/plain",
		Metadata:     map[string]string{"name": "Restart app"},
	}
	if err := s.PluginStorage.Put(ctx, item); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := s.PluginStorage.Get(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
		Key:          "prod/restart",
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Value) != "systemctl restart app" || got.Metadata["name"] != "Restart app" {
		t.Fatalf("stored item mismatch: %+v", got)
	}

	otherOwner := *item
	otherOwner.OwnerID = "u2"
	otherOwner.Value = []byte("other")
	if err := s.PluginStorage.Put(ctx, &otherOwner); err != nil {
		t.Fatalf("put other owner: %v", err)
	}
	if _, err := s.PluginStorage.Get(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
		Key:          "missing",
	}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get missing: want ErrNotFound, got %v", err)
	}

	second := *item
	second.ItemKey = "prod/status"
	second.Value = []byte("systemctl status app")
	if err := s.PluginStorage.Put(ctx, &second); err != nil {
		t.Fatalf("put second: %v", err)
	}
	rows, err := s.PluginStorage.List(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 || rows[0].ItemKey != "prod/restart" || rows[1].ItemKey != "prod/status" {
		t.Fatalf("unexpected filtered rows: %+v", rows)
	}

	item.Value = []byte("updated")
	item.Metadata = map[string]string{"name": "Updated"}
	if err := s.PluginStorage.Put(ctx, item); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = s.PluginStorage.Get(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
		Key:          "prod/restart",
	})
	if err != nil {
		t.Fatalf("get updated: %v", err)
	}
	if string(got.Value) != "updated" || got.Metadata["name"] != "Updated" {
		t.Fatalf("updated item mismatch: %+v", got)
	}

	if err := s.PluginStorage.Delete(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
		Key:          "prod/restart",
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.PluginStorage.Get(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
		Key:          "prod/restart",
	}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
	if err := s.PluginStorage.Delete(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
	}); err != nil {
		t.Fatalf("delete remaining scope: %v", err)
	}
	rows, err = s.PluginStorage.List(ctx, store.PluginStorageFilter{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: scope.ConnectionID,
		OwnerID:      scope.OwnerID,
	})
	if err != nil {
		t.Fatalf("list after scope delete: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("scope delete left rows: %+v", rows)
	}

	shared := &models.PluginStorageItem{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		ConnectionID: "c2",
		OwnerID:      scope.OwnerID,
		ItemKey:      "global/profile",
		Value:        []byte("shared"),
	}
	if err := s.PluginStorage.Put(ctx, shared); err != nil {
		t.Fatalf("put user-scoped: %v", err)
	}
	rows, err = s.PluginStorage.List(ctx, store.PluginStorageFilter{
		Namespace: scope.Namespace,
		Plugin:    scope.Plugin,
		OwnerID:   scope.OwnerID,
	})
	if err != nil {
		t.Fatalf("list user-scoped: %v", err)
	}
	if len(rows) != 1 || rows[0].ItemKey != "global/profile" {
		t.Fatalf("owner-scoped filter should cross connection dimension: %+v", rows)
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
	now := time.Now()
	for i := range 3 {
		e := &models.AuditEntry{
			ID: "a" + string(rune('0'+i)), Time: now.Add(time.Duration(i) * time.Second),
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
	removed, err := s.Audit.DeleteBefore(ctx, now.Add(1500*time.Millisecond))
	if err != nil {
		t.Fatalf("delete before: %v", err)
	}
	if removed != 2 {
		t.Errorf("delete before removed: want 2, got %d", removed)
	}
	remaining, _ := s.Audit.List(ctx, store.AuditFilter{ConnectionID: "c1"})
	if len(remaining) != 1 || remaining[0].ID != "a2" {
		t.Errorf("delete before remaining: %+v", remaining)
	}
}

func testRecordings(t *testing.T, s *store.Store) {
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	r := &models.Recording{
		ID: "rec1", UserID: "u1", Username: "alice", ConnectionID: "c1", ConnectionName: "prod",
		Protocol: "ssh", RouteID: "ssh.shell", StreamID: "ssh.shell", Class: "terminal",
		Format: "asciicast_v2", Authoritative: true, Status: models.RecordingActive,
		StartedAt: now, StorageKey: "c1/rec1.cast",
	}
	if err := s.Recordings.Create(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Recordings.Get(ctx, "rec1")
	if err != nil || got.Format != "asciicast_v2" || got.Class != "terminal" || !got.Authoritative {
		t.Fatalf("get round-trip: %+v err=%v", got, err)
	}

	ended := now.Add(5 * time.Second)
	got.Status = models.RecordingFinalized
	got.EndedAt = &ended
	got.Size = 4096
	got.Checksum = "abc123"
	got.ExpiresAt = &past
	if err := s.Recordings.Update(ctx, &got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, _ := s.Recordings.Get(ctx, "rec1")
	if reloaded.Status != models.RecordingFinalized || reloaded.Size != 4096 || reloaded.Checksum != "abc123" {
		t.Fatalf("update not persisted: %+v", reloaded)
	}

	// A second, non-expired recording for another user/connection.
	if err := s.Recordings.Create(ctx, &models.Recording{
		ID: "rec2", UserID: "u2", ConnectionID: "c2", Protocol: "ssh", Class: "terminal",
		Format: "asciicast_v2", Status: models.RecordingFinalized, StartedAt: now.Add(time.Minute),
		ExpiresAt: &future, StorageKey: "c2/rec2.cast",
	}); err != nil {
		t.Fatalf("create rec2: %v", err)
	}

	// Filter by user.
	if mine, _ := s.Recordings.List(ctx, store.RecordingFilter{UserID: "u1"}); len(mine) != 1 || mine[0].ID != "rec1" {
		t.Fatalf("filter by user: %+v", mine)
	}
	// Filter by connection.
	if byConn, _ := s.Recordings.List(ctx, store.RecordingFilter{ConnectionID: "c2"}); len(byConn) != 1 || byConn[0].ID != "rec2" {
		t.Fatalf("filter by connection: %+v", byConn)
	}
	// Expiry filter selects only the past-due one.
	expired, _ := s.Recordings.List(ctx, store.RecordingFilter{ExpiredBefore: now})
	if len(expired) != 1 || expired[0].ID != "rec1" {
		t.Fatalf("expired filter: want [rec1], got %+v", expired)
	}

	if err := s.Recordings.Delete(ctx, "rec1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Recordings.Get(ctx, "rec1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("get deleted: want ErrNotFound, got %v", err)
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
