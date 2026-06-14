package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type credentialCatalogPlugin struct{}

const (
	testCredentialKubeconfig plugin.CredentialKind = "kubeconfig"
)

func (credentialCatalogPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "catalog", Title: "Catalog", Category: plugin.CategoryOther,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		CredentialKinds: []plugin.CredentialKindInfo{
			{
				Kind: testCredentialKubeconfig, Label: "Kubeconfig",
				Fields: []plugin.Field{
					plugin.CredentialPublicField(plugin.Field{Key: "context", Label: "Context / user", Type: plugin.FieldText}),
					plugin.CredentialSecretField(plugin.Field{Key: "kubeconfig", Label: "Kubeconfig YAML", Type: plugin.FieldTextarea, Required: true}),
				},
			},
		},
		Config: plugin.Schema{Groups: []plugin.Group{{Name: "Auth", Fields: []plugin.Field{
			{
				Key: "ssh_key_credential", Label: "SSH key credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindSSHPrivateKey, Protocols: []string{"ssh"}},
			},
			{
				Key: "ssh_password_credential", Label: "SSH password credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindSSHPassword, Protocols: []string{"ssh"}},
			},
			{
				Key: "db_credential", Label: "Database credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindDBPassword, Protocols: []string{"postgres"}},
			},
			{
				Key: "api_credential", Label: "API credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindAPIToken, Protocols: []string{"http-api"}},
			},
			{
				Key: "kube_credential", Label: "Kube credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kind: testCredentialKubeconfig, Protocols: []string{"kubernetes"}},
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
	reg := pluginregistry.New()
	reg.MustRegister(credentialCatalogPlugin{})
	return service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg)), st
}

func TestCredentialCreateEncryptsAtRest(t *testing.T) {
	ctx := context.Background()
	svc, st := newCredentialService(t)

	cred, err := svc.Create(ctx, service.NewCredentialInput{
		OwnerID: "owner", Name: "ops", Kind: "ssh_password",
		Values: map[string]string{"username": "ops", "password": "hunter2"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	stored, _ := st.Credentials.Get(ctx, cred.ID)
	if len(stored.EncryptedValues) == 0 {
		t.Fatal("no encrypted secret stored")
	}
	if string(stored.EncryptedValues) == "hunter2" || containsBytes(stored.EncryptedValues, "hunter2") {
		t.Error("plaintext leaked into stored credential")
	}
	if len(stored.Protocols) != 1 || stored.Protocols[0] != "ssh" {
		t.Fatalf("stored protocols = %+v, want derived [ssh]", stored.Protocols)
	}
	sum := stored.Summary()
	if sum.ID != cred.ID || sum.Kind != "ssh_password" {
		t.Errorf("summary wrong: %+v", sum)
	}
	if sum.Values["username"] != "ops" {
		t.Errorf("summary values = %+v, want username", sum.Values)
	}
}

func TestCredentialResolveOwnerAndGrant(t *testing.T) {
	ctx := context.Background()
	svc, st := newCredentialService(t)
	cred, _ := svc.Create(ctx, service.NewCredentialInput{
		OwnerID: "owner", Name: "k", Kind: "ssh_password",
		Values: map[string]string{"username": "ops", "password": "topsecret"},
	})
	secretAccesses := 0
	svc.SetSecretAccessHook(func() { secretAccesses++ })

	_, values, err := svc.ResolveWithMetadata(ctx, "owner", cred.ID)
	if err != nil || values["password"] != "topsecret" || values["username"] != "ops" {
		t.Fatalf("owner resolve: values=%+v err=%v", values, err)
	}
	if secretAccesses != 1 {
		t.Fatalf("secret access hook calls = %d, want 1", secretAccesses)
	}

	if _, _, err := svc.ResolveWithMetadata(ctx, "stranger", cred.ID); !errors.Is(err, models.ErrForbidden) {
		t.Errorf("stranger resolve: want ErrForbidden, got %v", err)
	}
	if secretAccesses != 1 {
		t.Fatalf("forbidden resolve should not count secret access, got %d", secretAccesses)
	}

	_ = st.CredentialGrants.Create(ctx, &models.CredentialGrant{ID: "cg1", CredentialID: cred.ID, SubjectID: "stranger", Access: models.AccessUse})
	_, values, err = svc.ResolveWithMetadata(ctx, "stranger", cred.ID)
	if err != nil || values["password"] != "topsecret" {
		t.Errorf("granted resolve: values=%+v err=%v", values, err)
	}
	if secretAccesses != 2 {
		t.Fatalf("secret access hook calls = %d, want 2", secretAccesses)
	}
}

func TestCredentialRotateAndResolve(t *testing.T) {
	ctx := context.Background()
	svc, st := newCredentialService(t)
	cred, _ := svc.Create(ctx, service.NewCredentialInput{
		OwnerID: "owner", Name: "k", Kind: "ssh_password",
		Values: map[string]string{"username": "ops", "password": "old-secret"},
	})

	if _, values, err := svc.ResolveWithMetadata(ctx, "owner", cred.ID); err != nil || values["password"] != "old-secret" {
		t.Fatalf("initial resolve: values=%+v err=%v", values, err)
	}

	if _, err := svc.Update(ctx, cred.ID, service.UpdateCredentialInput{
		Name: "k", Kind: "ssh_password",
		Values: map[string]string{"username": "ops", "password": "new-secret"},
	}); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, values, err := svc.ResolveWithMetadata(ctx, "owner", cred.ID); err != nil || values["password"] != "new-secret" {
		t.Fatalf("post-rotate resolve: values=%+v err=%v", values, err)
	}

	if _, err := svc.Update(ctx, cred.ID, service.UpdateCredentialInput{
		Name: "renamed", Kind: "ssh_password",
		Values: map[string]string{"username": "ops"},
	}); err != nil {
		t.Fatalf("metadata-only update: %v", err)
	}
	if _, values, err := svc.ResolveWithMetadata(ctx, "owner", cred.ID); err != nil || values["password"] != "new-secret" {
		t.Fatalf("resolve after metadata update: values=%+v err=%v", values, err)
	}

	stored, _ := st.Credentials.Get(ctx, cred.ID)
	if stored.Name != "renamed" {
		t.Errorf("metadata not persisted: %+v", stored)
	}
	if containsBytes(stored.EncryptedValues, "new-secret") {
		t.Error("plaintext leaked into stored credential after rotation")
	}
}

func TestCredentialResolveNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newCredentialService(t)
	if _, _, err := svc.ResolveWithMetadata(ctx, "owner", "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing credential: want ErrNotFound, got %v", err)
	}
}

func TestCredentialListUsableFilters(t *testing.T) {
	ctx := context.Background()
	svc, _ := newCredentialService(t)
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "ssh-key", Kind: "ssh_private_key", Values: map[string]string{"username": "ops", "private_key": "a"}})
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "db-pw", Kind: "db_password", Values: map[string]string{"username": "db", "password": "b"}})
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "api-token", Kind: "api_token", Values: map[string]string{"subject": "svc", "token": "c"}})
	_, _ = svc.Create(ctx, service.NewCredentialInput{OwnerID: "u", Name: "kube", Kind: "kubeconfig", Values: map[string]string{"context": "prod", "kubeconfig": "d"}})

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
		OwnerID: "u", Name: "bad", Kind: "made_up", Values: map[string]string{"password": "x"},
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
