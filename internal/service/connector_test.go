package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/sdk/plugin"
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
				Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindAPIToken},
			},
		}}}},
		Tabs: []plugin.Panel{{Key: "main", Label: "Main", Type: plugin.PanelTable}},
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
	reg := pluginregistry.New()
	reg.MustRegister(credentialRefPlugin{})
	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg))

	cred, err := creds.Create(ctx, service.NewCredentialInput{
		OwnerID: "u1",
		Name:    "token",
		Kind:    "api_token",
		Values:  map[string]string{"subject": "svc-api", "token": "secret-token"},
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
	credValues, ok := cfg.CredentialFor("api_credential")
	if !ok {
		t.Fatalf("resolved credential missing: %+v", cfg.Credentials)
	}
	values := credValues.Values
	if values["token"] != "secret-token" || values["subject"] != "svc-api" {
		t.Fatalf("resolved credential values = %+v", values)
	}
	if got := credValues.Kind; got != plugin.CredentialKindAPIToken {
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
	reg := pluginregistry.New()
	reg.MustRegister(credentialRefPlugin{})
	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg))

	cred, err := creds.Create(ctx, service.NewCredentialInput{
		OwnerID: "owner",
		Name:    "token",
		Kind:    "api_token",
		Values:  map[string]string{"token": "owner-secret"},
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
	credValues, ok := cfg.CredentialFor("api_credential")
	if !ok || credValues.Value("token") != "owner-secret" {
		t.Fatalf("resolved credential values = %#v, want owner-secret token", cfg.Credentials)
	}
}

type secretHostPlugin struct{}

func (secretHostPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "secret-host",
		Version:             "0",
		Title:               "Secret Host",
		Category:            plugin.CategoryDevOps,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Config: plugin.Schema{Groups: []plugin.Group{{Name: "Conn", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Secret: true},
		}}}},
		Tabs: []plugin.Panel{{Key: "main", Label: "Main", Type: plugin.PanelTable}},
	}
}

func (secretHostPlugin) Routes() []plugin.Route { return nil }
func (secretHostPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, nil
}

// Secret values must not seed the direct transport's target allowlist: a
// password that happens to look like an address is not a dialable target.
func TestConnectorTransportAllowlistExcludesSecrets(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	key, _ := secrets.GenerateMasterKey()
	vault, _ := secrets.NewVault(key)
	reg := pluginregistry.New()
	reg.MustRegister(secretHostPlugin{})
	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg))

	enc, err := secrets.EncryptMap(ctx, vault, map[string]string{"password": "10.99.99.99"})
	if err != nil {
		t.Fatalf("encrypt secrets: %v", err)
	}

	connector := service.NewConnector(reg, creds, vault, transport.NewRegistry())
	cfg, _, err := connector.Build(ctx,
		models.User{ID: "u1"},
		models.Connection{
			ID:        "c1",
			Protocol:  "secret-host",
			Transport: string(plugin.TransportDirect),
			OwnerID:   "u1",
			Config:    map[string]any{"host": "127.0.0.1", "port": 1},
			Secrets:   enc,
		},
	)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	if got := cfg.Config["password"]; got != "10.99.99.99" {
		t.Fatalf("decrypted password should reach the plugin config, got %#v", got)
	}

	if _, err := cfg.Net.DialContext(ctx, "tcp", "10.99.99.99:80"); err == nil ||
		!strings.Contains(err.Error(), "outside connection target") {
		t.Fatalf("dial to secret-derived host should be rejected by the allowlist, got %v", err)
	}
	if _, err := cfg.Net.DialContext(ctx, "tcp", "127.0.0.1:1"); err != nil &&
		strings.Contains(err.Error(), "outside connection target") {
		t.Fatalf("declared host must stay allowed, got %v", err)
	}
}
