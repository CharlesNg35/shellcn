package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

var (
	// ErrAuthProviderNotFound signals the requested provider configuration is missing.
	ErrAuthProviderNotFound = errors.New("auth provider service: provider not found")
	// ErrAuthProviderImmutable prevents destructive operations on system providers.
	ErrAuthProviderImmutable = errors.New("auth provider service: provider cannot be modified")
)

// LDAPConnectionTester defines a connection validation function for LDAP providers.
type LDAPConnectionTester func(models.LDAPConfig) error

// OIDCConnectionTester defines a connection validation function for OIDC providers.
type OIDCConnectionTester func(models.OIDCConfig) error

// AuthProviderService manages authentication provider configurations.
type AuthProviderService struct {
	db            *gorm.DB
	auditService  *AuditService
	encryptionKey []byte
	ldapTester    LDAPConnectionTester
	oidcTester    OIDCConnectionTester
}

// PublicProvider represents metadata exposed to unauthenticated clients.
type PublicProvider struct {
	Type                     string `json:"type"`
	Name                     string `json:"name"`
	Description              string `json:"description"`
	Icon                     string `json:"icon"`
	Enabled                  bool   `json:"enabled"`
	AllowRegistration        bool   `json:"allow_registration"`
	RequireEmailVerification bool   `json:"require_email_verification"`
	AllowPasswordReset       bool   `json:"allow_password_reset"`
	Flow                     string `json:"flow"`
}

// NewAuthProviderService constructs an AuthProviderService instance.
func NewAuthProviderService(db *gorm.DB, auditService *AuditService, encryptionKey []byte) (*AuthProviderService, error) {
	if db == nil {
		return nil, errors.New("auth provider service: db is required")
	}
	if len(encryptionKey) != 16 && len(encryptionKey) != 24 && len(encryptionKey) != 32 {
		return nil, errors.New("auth provider service: encryption key must be 16, 24, or 32 bytes")
	}

	return &AuthProviderService{
		db:            db,
		auditService:  auditService,
		encryptionKey: encryptionKey,
		ldapTester:    defaultLDAPTester,
		oidcTester:    defaultOIDCTester,
	}, nil
}

// SetLDAPTester overrides the default LDAP connection tester (primarily for tests).
func (s *AuthProviderService) SetLDAPTester(tester LDAPConnectionTester) {
	if tester != nil {
		s.ldapTester = tester
	}
}

// SetOIDCTester overrides the default OIDC discovery tester (primarily for tests).
func (s *AuthProviderService) SetOIDCTester(tester OIDCConnectionTester) {
	if tester != nil {
		s.oidcTester = tester
	}
}

// List returns all providers with configuration payloads redacted.
func (s *AuthProviderService) List(ctx context.Context) ([]models.AuthProvider, error) {
	ctx = ensureContext(ctx)

	var providers []models.AuthProvider
	if err := s.db.WithContext(ctx).Order("type ASC").Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("auth provider service: list providers: %w", err)
	}

	for i := range providers {
		providers[i].Config = nil
	}

	return providers, nil
}

// GetByType returns the provider configuration for the supplied type.
func (s *AuthProviderService) GetByType(ctx context.Context, providerType string) (*models.AuthProvider, error) {
	ctx = ensureContext(ctx)

	var provider models.AuthProvider
	err := s.db.WithContext(ctx).First(&provider, "type = ?", providerType).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAuthProviderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth provider service: get provider: %w", err)
	}
	return &provider, nil
}

// GetEnabled returns all providers currently enabled for authentication.
func (s *AuthProviderService) GetEnabled(ctx context.Context) ([]models.AuthProvider, error) {
	ctx = ensureContext(ctx)

	var providers []models.AuthProvider
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("auth provider service: get enabled providers: %w", err)
	}

	for i := range providers {
		providers[i].Config = nil
	}

	return providers, nil
}

// GetEnabledPublic returns metadata for enabled providers, including mandatory local provider.
func (s *AuthProviderService) GetEnabledPublic(ctx context.Context) ([]PublicProvider, error) {
	ctx = ensureContext(ctx)

	var providers []models.AuthProvider
	if err := s.db.WithContext(ctx).
		Where("enabled = ? OR type = ?", true, "local").
		Order("type ASC").
		Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("auth provider service: get public providers: %w", err)
	}

	result := make([]PublicProvider, 0, len(providers))
	for _, provider := range providers {
		flow := "redirect"
		switch provider.Type {
		case "local", "ldap":
			flow = "password"
		}
		result = append(result, PublicProvider{
			Type:                     provider.Type,
			Name:                     provider.Name,
			Description:              provider.Description,
			Icon:                     provider.Icon,
			Enabled:                  provider.Enabled,
			AllowRegistration:        provider.AllowRegistration,
			RequireEmailVerification: provider.RequireEmailVerification,
			AllowPasswordReset:       provider.AllowPasswordReset,
			Flow:                     flow,
		})
	}
	return result, nil
}

// ConfigureOIDC upserts an OpenID Connect provider configuration.
func (s *AuthProviderService) ConfigureOIDC(ctx context.Context, cfg models.OIDCConfig, enabled bool, allowRegistration bool, createdBy string) error {
	ctx = ensureContext(ctx)

	cpy := cfg
	secret, err := crypto.Encrypt([]byte(cpy.ClientSecret), s.encryptionKey)
	if err != nil {
		return fmt.Errorf("auth provider service: encrypt oidc secret: %w", err)
	}
	cpy.ClientSecret = secret

	payload, err := json.Marshal(cpy)
	if err != nil {
		return fmt.Errorf("auth provider service: marshal oidc config: %w", err)
	}

	provider := models.AuthProvider{
		Type:              "oidc",
		Name:              "OpenID Connect",
		Enabled:           enabled,
		Config:            datatypes.JSON(payload),
		AllowRegistration: allowRegistration,
		Description:       "Single Sign-On via OpenID Connect",
		Icon:              "shield-check",
		CreatedBy:         strings.TrimSpace(createdBy),
	}

	if err := s.db.WithContext(ctx).
		Where("type = ?", provider.Type).
		Assign(provider).
		FirstOrCreate(&provider).Error; err != nil {
		return fmt.Errorf("auth provider service: upsert oidc provider: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "auth_provider.configure",
		Resource: provider.Type,
		Result:   "success",
		Metadata: map[string]any{"enabled": enabled},
	})

	return nil
}

// ConfigureSAML upserts a SAML provider configuration.
func (s *AuthProviderService) ConfigureSAML(ctx context.Context, cfg models.SAMLConfig, enabled bool, allowRegistration bool, createdBy string) error {
	ctx = ensureContext(ctx)

	cpy := cfg
	privateKey, err := crypto.Encrypt([]byte(cpy.PrivateKey), s.encryptionKey)
	if err != nil {
		return fmt.Errorf("auth provider service: encrypt saml private key: %w", err)
	}
	cpy.PrivateKey = privateKey

	payload, err := json.Marshal(cpy)
	if err != nil {
		return fmt.Errorf("auth provider service: marshal saml config: %w", err)
	}

	provider := models.AuthProvider{
		Type:              "saml",
		Name:              "SAML 2.0",
		Enabled:           enabled,
		Config:            datatypes.JSON(payload),
		AllowRegistration: allowRegistration,
		Description:       "SAML 2.0 Single Sign-On",
		Icon:              "shield",
		CreatedBy:         strings.TrimSpace(createdBy),
	}

	if err := s.db.WithContext(ctx).
		Where("type = ?", provider.Type).
		Assign(provider).
		FirstOrCreate(&provider).Error; err != nil {
		return fmt.Errorf("auth provider service: upsert saml provider: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "auth_provider.configure",
		Resource: provider.Type,
		Result:   "success",
		Metadata: map[string]any{"enabled": enabled},
	})

	return nil
}

// ConfigureLDAP upserts an LDAP provider configuration.
func (s *AuthProviderService) ConfigureLDAP(ctx context.Context, cfg models.LDAPConfig, enabled bool, allowRegistration bool, createdBy string) error {
	ctx = ensureContext(ctx)

	cpy := cfg
	password, err := crypto.Encrypt([]byte(cpy.BindPassword), s.encryptionKey)
	if err != nil {
		return fmt.Errorf("auth provider service: encrypt ldap password: %w", err)
	}
	cpy.BindPassword = password

	payload, err := json.Marshal(cpy)
	if err != nil {
		return fmt.Errorf("auth provider service: marshal ldap config: %w", err)
	}

	provider := models.AuthProvider{
		Type:              "ldap",
		Name:              "LDAP / Active Directory",
		Enabled:           enabled,
		Config:            datatypes.JSON(payload),
		AllowRegistration: allowRegistration,
		Description:       "LDAP or Active Directory authentication",
		Icon:              "building",
		CreatedBy:         strings.TrimSpace(createdBy),
	}

	if err := s.db.WithContext(ctx).
		Where("type = ?", provider.Type).
		Assign(provider).
		FirstOrCreate(&provider).Error; err != nil {
		return fmt.Errorf("auth provider service: upsert ldap provider: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "auth_provider.configure",
		Resource: provider.Type,
		Result:   "success",
		Metadata: map[string]any{"enabled": enabled},
	})

	return nil
}

// UpdateLocalSettings toggles options on the local provider.
func (s *AuthProviderService) UpdateLocalSettings(ctx context.Context, allowRegistration, requireEmailVerification, allowPasswordReset bool) error {
	ctx = ensureContext(ctx)

	updates := map[string]any{
		"allow_registration":         allowRegistration,
		"require_email_verification": requireEmailVerification,
		"allow_password_reset":       allowPasswordReset,
	}

	if err := s.db.WithContext(ctx).
		Model(&models.AuthProvider{}).
		Where("type = ?", "local").
		Updates(updates).Error; err != nil {
		return fmt.Errorf("auth provider service: update local settings: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "auth_provider.update",
		Resource: "local",
		Result:   "success",
		Metadata: updates,
	})

	return nil
}

// UpdateInviteSettings toggles the invite provider state.
func (s *AuthProviderService) UpdateInviteSettings(ctx context.Context, enabled, requireEmailVerification bool) error {
	ctx = ensureContext(ctx)

	updates := map[string]any{
		"enabled":                    enabled,
		"require_email_verification": requireEmailVerification,
	}

	if err := s.db.WithContext(ctx).
		Model(&models.AuthProvider{}).
		Where("type = ?", "invite").
		Updates(updates).Error; err != nil {
		return fmt.Errorf("auth provider service: update invite settings: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "auth_provider.update",
		Resource: "invite",
		Result:   "success",
		Metadata: updates,
	})

	return nil
}

// SetEnabled toggles a provider's enabled state while protecting system providers.
func (s *AuthProviderService) SetEnabled(ctx context.Context, providerType string, enabled bool) error {
	ctx = ensureContext(ctx)

	if providerType == "local" {
		return ErrAuthProviderImmutable
	}

	provider, err := s.GetByType(ctx, providerType)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Model(&models.AuthProvider{}).
		Where("type = ?", providerType).
		Update("enabled", enabled).Error; err != nil {
		return fmt.Errorf("auth provider service: set enabled: %w", err)
	}

	action := "auth_provider.enable"
	if !enabled {
		action = "auth_provider.disable"
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   action,
		Resource: provider.Type,
		Result:   "success",
	})

	return nil
}

// Delete removes non-system provider configurations.
func (s *AuthProviderService) Delete(ctx context.Context, providerType string) error {
	ctx = ensureContext(ctx)

	if providerType == "local" || providerType == "invite" {
		return ErrAuthProviderImmutable
	}

	if _, err := s.GetByType(ctx, providerType); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Where("type = ?", providerType).
		Delete(&models.AuthProvider{}).Error; err != nil {
		return fmt.Errorf("auth provider service: delete provider: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "auth_provider.delete",
		Resource: providerType,
		Result:   "success",
	})

	return nil
}

// TestConnection validates connectivity for providers that support it.
func (s *AuthProviderService) TestConnection(ctx context.Context, providerType string) error {
	ctx = ensureContext(ctx)

	provider, err := s.GetByType(ctx, providerType)
	if err != nil {
		return err
	}

	switch providerType {
	case "ldap":
		var cfg models.LDAPConfig
		if err := json.Unmarshal(provider.Config, &cfg); err != nil {
			return fmt.Errorf("auth provider service: decode ldap config: %w", err)
		}
		password, err := crypto.Decrypt(cfg.BindPassword, s.encryptionKey)
		if err != nil {
			return fmt.Errorf("auth provider service: decrypt ldap password: %w", err)
		}
		cfg.BindPassword = string(password)
		return s.ldapTester(cfg)
	case "oidc":
		var cfg models.OIDCConfig
		if err := json.Unmarshal(provider.Config, &cfg); err != nil {
			return fmt.Errorf("auth provider service: decode oidc config: %w", err)
		}
		secret, err := crypto.Decrypt(cfg.ClientSecret, s.encryptionKey)
		if err != nil {
			return fmt.Errorf("auth provider service: decrypt oidc secret: %w", err)
		}
		cfg.ClientSecret = string(secret)
		return s.oidcTester(cfg)
	default:
		return errors.New("auth provider service: connection testing not supported for provider")
	}
}

func defaultLDAPTester(cfg models.LDAPConfig) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("ldap tester: host is required")
	}
	if cfg.Port <= 0 {
		return errors.New("ldap tester: port must be positive")
	}
	if strings.TrimSpace(cfg.BaseDN) == "" {
		return errors.New("ldap tester: base DN is required")
	}
	return nil
}

func defaultOIDCTester(cfg models.OIDCConfig) error {
	if strings.TrimSpace(cfg.Issuer) == "" {
		return errors.New("oidc tester: issuer is required")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return errors.New("oidc tester: client id is required")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return errors.New("oidc tester: client secret is required")
	}
	if strings.TrimSpace(cfg.RedirectURL) == "" {
		return errors.New("oidc tester: redirect url is required")
	}
	return nil
}

// LoadOIDCConfig returns the decrypted OIDC configuration if the provider is configured.
func (s *AuthProviderService) LoadOIDCConfig(ctx context.Context) (*models.AuthProvider, *models.OIDCConfig, error) {
	ctx = ensureContext(ctx)

	provider, err := s.GetByType(ctx, "oidc")
	if err != nil {
		return nil, nil, err
	}
	if len(provider.Config) == 0 {
		return nil, nil, errors.New("auth provider service: oidc provider not configured")
	}

	var cfg models.OIDCConfig
	if err := json.Unmarshal(provider.Config, &cfg); err != nil {
		return nil, nil, fmt.Errorf("auth provider service: decode oidc config: %w", err)
	}

	secret, err := crypto.Decrypt(cfg.ClientSecret, s.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("auth provider service: decrypt oidc secret: %w", err)
	}
	cfg.ClientSecret = string(secret)

	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email"}
	}

	return provider, &cfg, nil
}

// LoadSAMLConfig returns the decrypted SAML configuration if configured.
func (s *AuthProviderService) LoadSAMLConfig(ctx context.Context) (*models.AuthProvider, *models.SAMLConfig, error) {
	ctx = ensureContext(ctx)

	provider, err := s.GetByType(ctx, "saml")
	if err != nil {
		return nil, nil, err
	}
	if len(provider.Config) == 0 {
		return nil, nil, errors.New("auth provider service: saml provider not configured")
	}

	var cfg models.SAMLConfig
	if err := json.Unmarshal(provider.Config, &cfg); err != nil {
		return nil, nil, fmt.Errorf("auth provider service: decode saml config: %w", err)
	}

	privateKey, err := crypto.Decrypt(cfg.PrivateKey, s.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("auth provider service: decrypt saml private key: %w", err)
	}
	cfg.PrivateKey = string(privateKey)

	return provider, &cfg, nil
}

// LoadLDAPConfig returns the decrypted LDAP configuration if configured.
func (s *AuthProviderService) LoadLDAPConfig(ctx context.Context) (*models.AuthProvider, *models.LDAPConfig, error) {
	ctx = ensureContext(ctx)

	provider, err := s.GetByType(ctx, "ldap")
	if err != nil {
		return nil, nil, err
	}
	if len(provider.Config) == 0 {
		return nil, nil, errors.New("auth provider service: ldap provider not configured")
	}

	var cfg models.LDAPConfig
	if err := json.Unmarshal(provider.Config, &cfg); err != nil {
		return nil, nil, fmt.Errorf("auth provider service: decode ldap config: %w", err)
	}

	password, err := crypto.Decrypt(cfg.BindPassword, s.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("auth provider service: decrypt ldap bind password: %w", err)
	}
	cfg.BindPassword = string(password)

	return provider, &cfg, nil
}
