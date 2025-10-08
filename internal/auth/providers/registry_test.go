package providers

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryRegisterAndList(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register(Descriptor{
		Metadata: Metadata{
			Type:         "oidc",
			DisplayName:  "OpenID Connect",
			ButtonText:   "Continue with OIDC",
			SupportsTest: true,
			Order:        20,
		},
		Factory: func(cfg ProviderConfig) (Provider, error) {
			return nil, nil
		},
	})
	require.NoError(t, err)

	err = reg.Register(Descriptor{
		Metadata: Metadata{
			Type:         "saml",
			DisplayName:  "SAML 2.0",
			ButtonText:   "Continue with SAML",
			SupportsTest: false,
			Order:        30,
		},
	})
	require.NoError(t, err)

	metadata := reg.Metadata()
	require.Len(t, metadata, 2)
	require.Equal(t, "oidc", metadata[0].Type)
	require.Equal(t, "saml", metadata[1].Type)

	factory, ok := reg.FactoryFor("OIDC")
	require.True(t, ok)
	require.NotNil(t, factory)

	instance, err := factory(ProviderConfig{
		Type:    "oidc",
		Name:    "OpenID Connect",
		Enabled: true,
		Raw:     json.RawMessage(`{"issuer":"https://example.com"}`),
	})
	require.NoError(t, err)
	require.Nil(t, instance)
}

func TestRegistryRejectsDuplicate(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register(Descriptor{Metadata: Metadata{Type: "ldap", DisplayName: "LDAP"}})
	require.NoError(t, err)

	err = reg.Register(Descriptor{Metadata: Metadata{Type: "ldap", DisplayName: "Duplicate LDAP"}})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrProviderExists))
}
