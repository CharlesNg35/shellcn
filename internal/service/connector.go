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

// CredentialField is the legacy/default config key holding a referenced
// credential id; CredentialSecret is the matching resolved plaintext key.
const (
	CredentialField    = "credential_id"
	CredentialSecret   = "_credential_secret"
	CredentialIdentity = "_credential_identity"
	CredentialKind     = "_credential_kind"
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
func (c *Connector) Build(ctx context.Context, _ models.User, conn models.Connection) (plugin.ConnectConfig, plugin.Plugin, error) {
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
					cred, material, err := c.creds.ResolveWithMetadata(ctx, conn.OwnerID, credID)
					if err != nil {
						return plugin.ConnectConfig{}, nil, fmt.Errorf("resolve credential: %w", err)
					}
					cfg[credentialSecretKey(key)] = string(material)
					cfg[credentialKindKey(key)] = cred.Kind
					if cred.Username != "" {
						cfg[credentialIdentityKey(key)] = cred.Username
					}
				}
			}
		}
	}

	transportConn := conn
	transportConn.Config = cfg
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
		Transport:    plugin.Transport(conn.Transport),
		Config:       cfg,
		Net:          net,
	}, plg, nil
}

func credentialSecretKey(key string) string {
	if key == CredentialField {
		return CredentialSecret
	}
	return "_" + key + "_secret"
}

func credentialIdentityKey(key string) string {
	if key == CredentialField {
		return CredentialIdentity
	}
	return "_" + key + "_identity"
}

func credentialKindKey(key string) string {
	if key == CredentialField {
		return CredentialKind
	}
	return "_" + key + "_kind"
}
