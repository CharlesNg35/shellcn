package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/secrets"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/internal/store"
)

type credentialCatalogPlugin struct{}

const (
	testCredentialSSHPrivateKey plugin.CredentialKind = "ssh_private_key"
	testCredentialSSHPassword   plugin.CredentialKind = "ssh_password"
	testCredentialKubeconfig    plugin.CredentialKind = "kubeconfig"
)

func (credentialCatalogPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "catalog", Title: "Catalog",
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		CredentialKinds: []plugin.CredentialKindInfo{
			{
				Kind: testCredentialSSHPrivateKey, Label: "SSH private key", SecretLabel: "Private key",
				SecretMultiline: true, IdentityLabel: "Username",
			},
			{
				Kind: testCredentialSSHPassword, Label: "SSH password", SecretLabel: "Password",
				IdentityLabel: "Username",
			},
			{
				Kind: testCredentialKubeconfig, Label: "Kubeconfig", SecretLabel: "Kubeconfig YAML",
				SecretMultiline: true, IdentityLabel: "Context / user",
			},
		},
		Config: plugin.Schema{Groups: []plugin.Group{{Name: "Auth", Fields: []plugin.Field{
			{
				Key: "ssh_credential", Label: "SSH credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{
					Kinds: []plugin.CredentialKind{testCredentialSSHPrivateKey, testCredentialSSHPassword}, Protocols: []string{"ssh"},
				},
			},
			{
				Key: "db_credential", Label: "Database credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kinds: []plugin.CredentialKind{plugin.CredentialDBPassword}, Protocols: []string{"postgres"}},
			},
			{
				Key: "api_credential", Label: "API credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kinds: []plugin.CredentialKind{plugin.CredentialAPIToken}, Protocols: []string{"http-api"}},
			},
			{
				Key: "kube_credential", Label: "Kube credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kinds: []plugin.CredentialKind{testCredentialKubeconfig}, Protocols: []string{"kubernetes"}},
			},
		}}}},
	}
}

func (credentialCatalogPlugin) Routes() []plugin.Route {
	return []plugin.Route{{ID: "catalog.list", Method: plugin.MethodGet, Permission: "catalog.read", Risk: plugin.RiskSafe, Handle: func(*plugin.RequestContext) (any, error) { return nil, nil }}}
}

func (credentialCatalogPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, nil
}

func newCredentialService(t *testing.T) (*service.CredentialService, *store.Store) {
	t.Helper()
	key, _ := secrets.GenerateMasterKey()
	vault, err := secrets.NewVault(key)
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	st := store.NewMemory()
	reg := plugin.NewRegistry()
	reg.MustRegister(credentialCatalogPlugin{})
	return service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg)), st
}

func TestCredentialCreateEncryptsAtRest(t *testing.T) {
	ctx := context.Background()
	svc, st := newCredentialService(t)

	cred, err := svc.Create(ctx, service.NewCredentialInput{
		OwnerID: "owner", Name: "ops", Kind: "ssh_password", Secret: "hunter2",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Stored material is ciphertext, never the plaintext.
	stored, _ := st.Credentials.Get(ctx, cred.ID)
	if len(stored.EncryptedSecret) == 0 {
		t.Fatal("no encrypted secret stored")
	}
	if string(stored.EncryptedSecret) == "hunter2" || containsBytes(stored.EncryptedSecret, "hunter2") {
		t.Error("plaintext leaked into stored credential")
	}
	if len(stored.Protocols) != 1 || stored.Protocols[0] != "ssh" {
		t.Fatalf("stored protocols = %+v, want derived [ssh]", stored.Protocols)
	}
	// The summary never carries secret material.
	sum := stored.Summary()
	if sum.ID != cred.ID || sum.Kind != "ssh_password" {
		t.Errorf("summary wrong: %+v", sum)
	}
}

func TestCredentialResolveOwnerAndGrant(t *testing.T) {
	ctx := context.Background()
	svc, st := newCredentialService(t)
	cred, _ := svc.Create(ctx, service.NewCredentialInput{OwnerID: "owner", Name: "k", Kind: "ssh_password", Secret: "topsecret"})
	secretAccesses := 0
	svc.SetSecretAccessHook(func() { secretAccesses++ })

	// Owner resolves the plaintext for connect-time injection.
	pt, err := svc.Resolve(ctx, "owner", cred.ID)
	if err != nil || string(pt) != "topsecret" {
		t.Fatalf("owner resolve: pt=%q err=%v", pt, err)
	}
	if secretAccesses != 1 {
		t.Fatalf("secret access hook calls = %d, want 1", secretAccesses)
	}

	// A stranger is forbidden.
	if _, err := svc.Resolve(ctx, "stranger", cred.ID); !errors.Is(err, models.ErrForbidden) {
		t.Errorf("stranger resolve: want ErrForbidden, got %v", err)
	}
	if secretAccesses != 1 {
		t.Fatalf("forbidden resolve should not count secret access, got %d", secretAccesses)
	}

	// After a use-grant, the grantee resolves it (still never readable as a value to the client).
	_ = st.CredentialGrants.Create(ctx, &models.CredentialGrant{ID: "cg1", CredentialID: cred.ID, SubjectID: "stranger", Access: models.AccessUse})
	pt, err = svc.Resolve(ctx, "stranger", cred.ID)
	if err != nil || string(pt) != "topsecret" {
		t.Errorf("granted resolve: pt=%q err=%v", pt, err)
	}
	if secretAccesses != 2 {
		t.Fatalf("secret access hook calls = %d, want 2", secretAccesses)
	}
}

func TestCredentialRotateAndResolve(t *testing.T) {
	ctx := context.Background()
	svc, st := newCredentialService(t)
	cred, _ := svc.Create(ctx, service.NewCredentialInput{OwnerID: "owner", Name: "k", Kind: "ssh_password", Secret: "old-secret"})

	if pt, err := svc.Resolve(ctx, "owner", cred.ID); err != nil || string(pt) != "old-secret" {
		t.Fatalf("initial resolve: pt=%q err=%v", pt, err)
	}

	// Rotate: every referencing connection picks up the new value on next resolve.
	if _, err := svc.Update(ctx, cred.ID, service.UpdateCredentialInput{Name: "k", Kind: "ssh_password", Secret: "new-secret"}); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if pt, err := svc.Resolve(ctx, "owner", cred.ID); err != nil || string(pt) != "new-secret" {
		t.Fatalf("post-rotate resolve: pt=%q err=%v", pt, err)
	}

	// A blank secret on update keeps the current material (write-only).
	if _, err := svc.Update(ctx, cred.ID, service.UpdateCredentialInput{Name: "renamed", Kind: "ssh_password", Secret: ""}); err != nil {
		t.Fatalf("metadata-only update: %v", err)
	}
	if pt, err := svc.Resolve(ctx, "owner", cred.ID); err != nil || string(pt) != "new-secret" {
		t.Fatalf("resolve after metadata update: pt=%q err=%v", pt, err)
	}

	stored, _ := st.Credentials.Get(ctx, cred.ID)
	if stored.Name != "renamed" {
		t.Errorf("metadata not persisted: %+v", stored)
	}
	if containsBytes(stored.EncryptedSecret, "new-secret") {
		t.Error("plaintext leaked into stored credential after rotation")
	}
}

func TestCredentialResolveNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newCredentialService(t)
	if _, err := svc.Resolve(ctx, "owner", "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing credential: want ErrNotFound, got %v", err)
	}
}

func TestCredentialListUsableFilters(t *testing.T) {
	ctx := context.Background()
	svc, _ := newCredentialService(t)
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "ssh-key", Kind: "ssh_private_key", Secret: "a"})
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "db-pw", Kind: "db_password", Secret: "b"})
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "api-token", Kind: "api_token", Secret: "c"})
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "kube", Kind: "kubeconfig", Secret: "d"})

	// Filter by kind.
	got, err := svc.ListUsable(ctx, "u", []string{"ssh_private_key"}, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Kind != "ssh_private_key" {
		t.Errorf("kind filter: %+v", got)
	}

	// Filter by protocol uses kind-derived protocol compatibility.
	got, _ = svc.ListUsable(ctx, "u", nil, "postgres")
	kinds := map[string]bool{}
	for _, c := range got {
		kinds[c.Kind] = true
	}
	if !kinds["db_password"] || kinds["api_token"] || kinds["ssh_private_key"] {
		t.Errorf("protocol filter wrong: %+v", got)
	}
	if kinds["kubeconfig"] {
		t.Errorf("incompatible wildcard kind should not match postgres: %+v", got)
	}

	// A summary never leaks anything secret.
	for _, c := range got {
		if c.Name == "" {
			t.Errorf("summary corrupted: %+v", c)
		}
	}
}

func TestCredentialCreateValidatesKindAndSecret(t *testing.T) {
	ctx := context.Background()
	svc, _ := newCredentialService(t)

	if _, err := svc.Create(ctx, service.NewCredentialInput{
		OwnerID: "u", Name: "bad", Kind: "made_up", Secret: "x",
	}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unknown kind: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Create(ctx, service.NewCredentialInput{
		OwnerID: "u", Name: "empty", Kind: "ssh_password",
	}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("empty secret on create: want ErrInvalidInput, got %v", err)
	}
}

func containsBytes(haystack []byte, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOfBytes(haystack, needle) >= 0
}

func indexOfBytes(h []byte, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if string(h[i:i+len(n)]) == n {
			return i
		}
	}
	return -1
}
