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

func TestRegistryNormalisesMetadataAndDefaults(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register(Descriptor{
		Metadata: Metadata{
			Type:        "  SAML  ",
			DisplayName: "  Sample SAML Provider  ",
			Description: "  description  ",
			ButtonText:  "  Sign In  ",
		},
		Factory: func(cfg ProviderConfig) (Provider, error) {
			return nil, nil
		},
	})
	require.NoError(t, err)

	meta := reg.Metadata()
	require.Len(t, meta, 1)
	require.Equal(t, "saml", meta[0].Type)
	require.Equal(t, "Sample SAML Provider", meta[0].DisplayName)
	require.Equal(t, "description", meta[0].Description)
	require.Equal(t, "Sign In", meta[0].ButtonText)
	require.Equal(t, 100, meta[0].Order, "expected default order applied")
}

func TestRegistrySortingByDisplayNameWhenOrderEqual(t *testing.T) {
	reg := NewRegistry()

	require.NoError(t, reg.Register(Descriptor{Metadata: Metadata{Type: "oidc", DisplayName: "OpenID", Order: 10}}))
	require.NoError(t, reg.Register(Descriptor{Metadata: Metadata{Type: "ldap", DisplayName: "Active Directory", Order: 10}}))

	meta := reg.Metadata()
	require.Len(t, meta, 2)
	require.Equal(t, "Active Directory", meta[0].DisplayName)
	require.Equal(t, "OpenID", meta[1].DisplayName)
}

func TestRegistryFactoryForMissingOrNil(t *testing.T) {
	reg := NewRegistry()

	require.NoError(t, reg.Register(Descriptor{
		Metadata: Metadata{Type: "oidc", DisplayName: "OIDC"},
		Factory: func(cfg ProviderConfig) (Provider, error) {
			return nil, nil
		},
	}))
	require.NoError(t, reg.Register(Descriptor{
		Metadata: Metadata{Type: "saml", DisplayName: "SAML"},
		Factory:  nil,
	}))

	_, ok := reg.FactoryFor("unknown")
	require.False(t, ok)

	_, ok = reg.FactoryFor("saml")
	require.False(t, ok, "expected false when factory is nil")

	factory, ok := reg.FactoryFor("OIDC")
	require.True(t, ok)
	require.NotNil(t, factory)
}

func TestRegistryRejectsMissingType(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register(Descriptor{Metadata: Metadata{DisplayName: "No Type"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "metadata type is required")
}
