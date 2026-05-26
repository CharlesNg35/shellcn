package service_test

import (
	"context"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/secrets"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/internal/store"
	"github.com/charlesng/shellcn/internal/transport"
)

type credentialRefPlugin struct{}

func (credentialRefPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "http-api",
		Version:    "0",
		Title:      "Credential Ref",
		Category:   plugin.CategoryDevOps,
		Layout:     plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{
			plugin.TransportDirect,
		},
		Config: plugin.Schema{Groups: []plugin.Group{{Name: "Auth", Fields: []plugin.Field{
			{
				Key: "api_credential", Label: "API Credential", Type: plugin.FieldCredentialRef,
				Credential: &plugin.CredentialSelector{Kinds: []plugin.CredentialKind{plugin.CredentialAPIToken}},
			},
		}}}},
		Tabs: []plugin.Tab{{Key: "main", Label: "Main", Panel: plugin.PanelTable}},
	}
}

func (credentialRefPlugin) Routes() []plugin.Route { return nil }
func (credentialRefPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, nil
}

func TestConnectorResolvesCredentialRefFieldsFromSchema(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)
	reg := plugin.NewRegistry()
	reg.MustRegister(credentialRefPlugin{})
	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg))

	cred, err := creds.Create(ctx, service.NewCredentialInput{
		OwnerID:  "u1",
		Name:     "token",
		Kind:     "api_token",
		Identity: "svc-api",
		Secret:   "secret-token",
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	connector := service.NewConnector(reg, creds, vault, transport.NewRegistry())
	cfg, _, err := connector.Build(ctx,
		models.User{ID: "u1"},
		models.Connection{
			ID:        "c1",
			Protocol:  "http-api",
			Transport: string(plugin.TransportDirect),
			OwnerID:   "u1",
			Config:    map[string]any{"api_credential": cred.ID},
		},
	)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	if got := cfg.Config["_api_credential_secret"]; got != "secret-token" {
		t.Fatalf("resolved credential secret = %#v, want secret-token", got)
	}
	if got := cfg.Config["_api_credential_identity"]; got != "svc-api" {
		t.Fatalf("resolved credential identity = %#v, want svc-api", got)
	}
	if got := cfg.Config["_api_credential_kind"]; got != "api_token" {
		t.Fatalf("resolved credential kind = %#v, want api_token", got)
	}
	if got := cfg.Config["api_credential"]; got != cred.ID {
		t.Fatalf("credential id field should remain stored id, got %#v", got)
	}
}

func TestConnectorResolvesSharedConnectionCredentialAsConnectionOwner(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)
	reg := plugin.NewRegistry()
	reg.MustRegister(credentialRefPlugin{})
	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg))

	cred, err := creds.Create(ctx, service.NewCredentialInput{
		OwnerID: "owner",
		Name:    "token",
		Kind:    "api_token",
		Secret:  "owner-secret",
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	connector := service.NewConnector(reg, creds, vault, transport.NewRegistry())
	cfg, _, err := connector.Build(ctx,
		models.User{ID: "viewer"},
		models.Connection{
			ID:        "c1",
			Protocol:  "http-api",
			Transport: string(plugin.TransportDirect),
			OwnerID:   "owner",
			Config:    map[string]any{"api_credential": cred.ID},
		},
	)
	if err != nil {
		t.Fatalf("shared connection should resolve owner credential: %v", err)
	}
	if got := cfg.Config["_api_credential_secret"]; got != "owner-secret" {
		t.Fatalf("resolved credential secret = %#v, want owner-secret", got)
	}
}
