package service

import (
	"context"
	"fmt"
	"maps"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/secrets"
	"github.com/charlesng/shellcn/internal/transport"
)

// CredentialField is the well-known config key holding a referenced credential's
// id; the connector resolves it to plaintext material under CredentialSecret.
const (
	CredentialField  = "credential_id"
	CredentialSecret = "_credential_secret"
)

// Connector assembles a plugin.ConnectConfig for a connection: it decrypts inline
// secrets, resolves any referenced credential (authorized for the acting user),
// and wires the transport for the connection's mode. Secret material lives only
// in the returned ConnectConfig — it is never serialized back to the client.
type Connector struct {
	plugins        *plugin.Registry
	creds          *CredentialService
	vault          secrets.SecretStore
	tunnels        transport.TunnelRegistry
	onSecretAccess func()
}

// NewConnector wires the dependencies.
func NewConnector(plugins *plugin.Registry, creds *CredentialService, vault secrets.SecretStore, tunnels transport.TunnelRegistry) *Connector {
	return &Connector{plugins: plugins, creds: creds, vault: vault, tunnels: tunnels}
}

// SetSecretAccessHook registers a callback for successful inline secret decryptions.
func (c *Connector) SetSecretAccessHook(fn func()) {
	c.onSecretAccess = fn
}

// Plugin resolves the plugin singleton for a connection's protocol.
func (c *Connector) Plugin(conn models.Connection) (plugin.Plugin, bool) {
	return c.plugins.Get(conn.Protocol)
}

// Build produces the ConnectConfig + plugin for a connection on behalf of user.
func (c *Connector) Build(ctx context.Context, user models.User, conn models.Connection) (plugin.ConnectConfig, plugin.Plugin, error) {
	plg, ok := c.plugins.Get(conn.Protocol)
	if !ok {
		return plugin.ConnectConfig{}, nil, fmt.Errorf("%w: protocol %q", plugin.ErrNotFound, conn.Protocol)
	}

	cfg := map[string]any{}
	maps.Copy(cfg, conn.Config)

	// Decrypt inline secrets into the config.
	inline, err := secrets.DecryptMap(ctx, c.vault, conn.Secrets)
	if err != nil {
		return plugin.ConnectConfig{}, nil, fmt.Errorf("decrypt inline secrets: %w", err)
	}
	for k, v := range inline {
		cfg[k] = v
	}
	if len(inline) > 0 && c.onSecretAccess != nil {
		c.onSecretAccess()
	}

	// Resolve a referenced reusable credential (authorized for this user).
	if credID, _ := cfg[CredentialField].(string); credID != "" {
		material, err := c.creds.Resolve(ctx, user.ID, credID)
		if err != nil {
			return plugin.ConnectConfig{}, nil, fmt.Errorf("resolve credential: %w", err)
		}
		cfg[CredentialSecret] = string(material)
	}

	net, err := transport.Build(conn, c.tunnels)
	if err != nil {
		return plugin.ConnectConfig{}, nil, err
	}

	return plugin.ConnectConfig{
		ConnectionID: conn.ID,
		Transport:    plugin.Transport(conn.Transport),
		Config:       cfg,
		Net:          net,
	}, plg, nil
}
