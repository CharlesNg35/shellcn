package service

import (
	"context"
	"fmt"
	"maps"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Connector assembles a plugin.ConnectConfig for a connection: it decrypts inline
// secrets, resolves any referenced credential (authorized for the acting user),
// and wires the transport for the connection's mode. Secret material lives only
// in the returned ConnectConfig — it is never serialized back to the client.
type Connector struct {
	plugins        *pluginregistry.Registry
	creds          *CredentialService
	vault          secrets.SecretStore
	tunnels        transport.TunnelRegistry
	onSecretAccess func()
}

func NewConnector(plugins *pluginregistry.Registry, creds *CredentialService, vault secrets.SecretStore, tunnels transport.TunnelRegistry) *Connector {
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
	manifest, hasManifest := c.plugins.Manifest(conn.Protocol)
	if hasManifest {
		context := connectionSchemaContext(conn.Protocol, conn.Transport)
		configWithDefaults := manifest.Config.ValuesWithDefaults(conn.Config)
		maps.Copy(cfg, manifest.Config.VisibleValues(configWithDefaults, context))
	} else {
		maps.Copy(cfg, conn.Config)
	}
	// The transport's target allowlist derives from the connection's declared
	// (non-secret) fields only — secret material must never seed dialable hosts.
	transportCfg := maps.Clone(cfg)

	// Decrypt inline secrets into the config.
	inline, err := secrets.DecryptMap(ctx, c.vault, conn.Secrets)
	if err != nil {
		return plugin.ConnectConfig{}, nil, fmt.Errorf("decrypt inline secrets: %w", err)
	}
	usedInline := false
	if hasManifest {
		context := connectionSchemaContext(conn.Protocol, conn.Transport)
		configWithDefaults := manifest.Config.ValuesWithDefaults(conn.Config)
		visibleSecrets := stringSet(manifest.Config.VisibleSecretKeys(configWithDefaults, context))
		for k, v := range inline {
			if visibleSecrets[k] {
				cfg[k] = v
				usedInline = true
			}
		}
	} else {
		for k, v := range inline {
			cfg[k] = v
			usedInline = true
		}
	}
	if usedInline && c.onSecretAccess != nil {
		c.onSecretAccess()
	}

	credentialBindings := []plugin.CredentialBinding{}
	// Resolve referenced reusable credentials through the connection owner. The
	// route wrapper has already authorized the acting user against the connection;
	// credential records remain hidden unless separately shared.
	if hasManifest {
		for _, group := range manifest.Config.Groups {
			for _, field := range group.Fields {
				if field.Type != plugin.FieldCredentialRef {
					continue
				}
				key := field.Key
				if credID, _ := cfg[key].(string); credID != "" {
					if err := c.creds.EnsureUsableFor(ctx, conn.OwnerID, credID, credentialSelectorKinds(field.Credential), conn.Protocol); err != nil {
						return plugin.ConnectConfig{}, nil, fmt.Errorf("resolve credential: %w", err)
					}
					cred, values, err := c.creds.ResolveWithMetadata(ctx, conn.OwnerID, credID)
					if err != nil {
						return plugin.ConnectConfig{}, nil, fmt.Errorf("resolve credential: %w", err)
					}
					credentialBindings = append(credentialBindings, plugin.CredentialBinding{
						Field: key,
						Credential: plugin.ResolvedCredential{
							ID:     cred.ID,
							Kind:   plugin.CredentialKind(cred.Kind),
							Values: values,
						},
					})
				}
			}
		}
	}

	transportConn := conn
	transportConn.Config = transportCfg
	var agentMode plugin.AgentMode
	if hasManifest && manifest.Agent != nil {
		agentMode = manifest.Agent.Proxy.Mode
	}
	net, err := transport.Build(transportConn, c.tunnels, agentMode)
	if err != nil {
		return plugin.ConnectConfig{}, nil, err
	}

	return plugin.ConnectConfig{
		ConnectionID: conn.ID,
		UserID:       user.ID,
		Transport:    plugin.Transport(conn.Transport),
		Config:       cfg,
		Credentials:  plugin.NewResolvedCredentials(credentialBindings...),
		Net:          net,
	}, plg, nil
}
