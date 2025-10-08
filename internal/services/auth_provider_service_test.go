package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

var testEncryptionKey = []byte("0123456789abcdef0123456789abcdef")

func TestAuthProviderServiceConfigureAndTestConnection(t *testing.T) {
	db := openAuthProviderServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	svc, err := NewAuthProviderService(db, auditSvc, testEncryptionKey)
	require.NoError(t, err)

	ctx := context.Background()

	require.NoError(t, db.Create(&models.AuthProvider{
		Type:                     "local",
		Name:                     "Local",
		Enabled:                  true,
		RequireEmailVerification: true,
	}).Error)
	require.NoError(t, db.Create(&models.AuthProvider{
		Type: "invite",
		Name: "Invite",
	}).Error)

	var capturedOIDC models.OIDCConfig
	svc.SetOIDCTester(func(cfg models.OIDCConfig) error {
		capturedOIDC = cfg
		return nil
	})

	err = svc.ConfigureOIDC(ctx, models.OIDCConfig{
		Issuer:       "https://idp.example.com",
		ClientID:     "client-id",
		ClientSecret: "super-secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile"},
	}, true, true, "admin")
	require.NoError(t, err)

	provider, err := svc.GetByType(ctx, "oidc")
	require.NoError(t, err)

	var storedCfg models.OIDCConfig
	require.NoError(t, json.Unmarshal([]byte(provider.Config), &storedCfg))
	require.NotEqual(t, "super-secret", storedCfg.ClientSecret)

	err = svc.TestConnection(ctx, "oidc")
	require.NoError(t, err)
	require.Equal(t, "super-secret", capturedOIDC.ClientSecret)

	var capturedLDAP models.LDAPConfig
	svc.SetLDAPTester(func(cfg models.LDAPConfig) error {
		capturedLDAP = cfg
		return nil
	})

	err = svc.ConfigureLDAP(ctx, models.LDAPConfig{
		Host:         "ldap.example.com",
		Port:         389,
		BaseDN:       "dc=example,dc=com",
		BindDN:       "cn=admin,dc=example,dc=com",
		BindPassword: "bind-secret",
		UserFilter:   "(uid={username})",
	}, true, false, "admin")
	require.NoError(t, err)

	err = svc.TestConnection(ctx, "ldap")
	require.NoError(t, err)
	require.Equal(t, "bind-secret", capturedLDAP.BindPassword)

	list, err := svc.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, list)
	for _, p := range list {
		require.Empty(t, p.Config)
	}
}

func TestAuthProviderServiceMutations(t *testing.T) {
	db := openAuthProviderServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	svc, err := NewAuthProviderService(db, auditSvc, testEncryptionKey)
	require.NoError(t, err)

	ctx := context.Background()

	require.NoError(t, db.Create(&models.AuthProvider{
		Type:                     "local",
		Name:                     "Local",
		Enabled:                  true,
		RequireEmailVerification: true,
	}).Error)
	require.NoError(t, db.Create(&models.AuthProvider{
		Type: "invite",
		Name: "Invite",
	}).Error)

	err = svc.UpdateLocalSettings(ctx, false, false)
	require.NoError(t, err)

	local, err := svc.GetByType(ctx, "local")
	require.NoError(t, err)
	require.False(t, local.AllowRegistration)
	require.False(t, local.RequireEmailVerification)

	err = svc.UpdateInviteSettings(ctx, true, true)
	require.NoError(t, err)

	invite, err := svc.GetByType(ctx, "invite")
	require.NoError(t, err)
	require.True(t, invite.Enabled)

	require.ErrorIs(t, svc.SetEnabled(ctx, "local", false), ErrAuthProviderImmutable)
	require.ErrorIs(t, svc.Delete(ctx, "invite"), ErrAuthProviderImmutable)
}

func TestAuthProviderServicePublicAndLoadConfig(t *testing.T) {
	db := openAuthProviderServiceTestDB(t)
	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	svc, err := NewAuthProviderService(db, auditSvc, testEncryptionKey)
	require.NoError(t, err)

	ctx := context.Background()

	require.NoError(t, db.Create(&models.AuthProvider{
		Type:                     "local",
		Name:                     "Local Authentication",
		Enabled:                  true,
		AllowRegistration:        true,
		RequireEmailVerification: true,
		Description:              "Local",
		Icon:                     "key",
	}).Error)

	err = svc.ConfigureOIDC(ctx, models.OIDCConfig{
		Issuer:       "https://idp.example.com",
		ClientID:     "client-id",
		ClientSecret: "super-secret",
		RedirectURL:  "https://shellcn.example.com/api/auth/providers/oidc/callback",
		Scopes:       []string{"openid", "profile"},
	}, true, true, "admin-user")
	require.NoError(t, err)

	err = svc.ConfigureSAML(ctx, models.SAMLConfig{
		MetadataURL: "",
		EntityID:    "https://sp.example.com/metadata",
		SSOURL:      "https://idp.example.com/sso",
		ACSURL:      "https://shellcn.example.com/api/auth/providers/saml/callback",
		Certificate: "-----BEGIN CERTIFICATE-----\nMIIBijCCAS+gAwIBAgIRAI8l\n-----END CERTIFICATE-----",
		PrivateKey:  "dummy",
		AttributeMapping: map[string]string{
			"email": "mail",
		},
	}, true, true, "admin-user")
	require.NoError(t, err)

	err = svc.ConfigureLDAP(ctx, models.LDAPConfig{
		Host:         "ldap.example.com",
		Port:         636,
		BaseDN:       "dc=example,dc=com",
		BindDN:       "cn=admin,dc=example,dc=com",
		BindPassword: "secret",
		UserFilter:   "(uid={username})",
		UseTLS:       true,
	}, true, true, "admin-user")
	require.NoError(t, err)

	publicProviders, err := svc.GetEnabledPublic(ctx)
	require.NoError(t, err)
	require.Len(t, publicProviders, 4)

	var localFound, oidcFound, samlFound, ldapFound bool
	for _, p := range publicProviders {
		switch p.Type {
		case "local":
			localFound = true
			require.True(t, p.AllowRegistration)
			require.Equal(t, "password", p.Flow)
		case "oidc":
			oidcFound = true
			require.True(t, p.Enabled)
			require.Equal(t, "redirect", p.Flow)
		case "saml":
			samlFound = true
			require.Equal(t, "redirect", p.Flow)
		case "ldap":
			ldapFound = true
			require.Equal(t, "password", p.Flow)
		}
	}
	require.True(t, localFound)
	require.True(t, oidcFound)
	require.True(t, samlFound)
	require.True(t, ldapFound)

	providerModel, cfg, err := svc.LoadOIDCConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "oidc", providerModel.Type)
	require.Equal(t, "client-id", cfg.ClientID)
	require.Equal(t, "super-secret", cfg.ClientSecret)
	require.Contains(t, cfg.Scopes, "openid")

	samlProvider, samlCfg, err := svc.LoadSAMLConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "saml", samlProvider.Type)
	require.Equal(t, "dummy", samlCfg.PrivateKey)
	require.Equal(t, "https://shellcn.example.com/api/auth/providers/saml/callback", samlCfg.ACSURL)

	ldapProvider, ldapCfg, err := svc.LoadLDAPConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "ldap", ldapProvider.Type)
	require.Equal(t, "secret", ldapCfg.BindPassword)
}

func openAuthProviderServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.AuthProvider{},
		&models.AuditLog{},
	))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
