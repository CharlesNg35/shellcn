package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

var (
	// ErrSSOEmailRequired indicates the upstream identity did not supply an email address and provisioning is not possible.
	ErrSSOEmailRequired = errors.New("sso manager: email is required")
	// ErrSSOUserNotFound is returned when the identity cannot be mapped to an existing user and auto-provisioning is disabled.
	ErrSSOUserNotFound = errors.New("sso manager: user not found")
	// ErrSSOUserDisabled signals that the mapped account is inactive.
	ErrSSOUserDisabled = errors.New("sso manager: user disabled")
)

// SSOConfig exposes tunable behaviour for the SSOManager.
type SSOConfig struct {
	Clock func() time.Time
}

// ResolveOptions customises how external identities are linked to local users.
type ResolveOptions struct {
	AutoProvision bool
	SessionMeta   SessionMetadata
}

// SSOManager coordinates mapping external provider identities to local users and issuing sessions.
type SSOManager struct {
	db       *gorm.DB
	sessions *SessionService
	clock    func() time.Time
}

// NewSSOManager constructs an SSOManager.
func NewSSOManager(db *gorm.DB, sessions *SessionService, cfg SSOConfig) (*SSOManager, error) {
	if db == nil {
		return nil, errors.New("sso manager: db is required")
	}
	if sessions == nil {
		return nil, errors.New("sso manager: session service is required")
	}

	clock := time.Now
	if cfg.Clock != nil {
		clock = cfg.Clock
	}

	return &SSOManager{
		db:       db,
		sessions: sessions,
		clock:    clock,
	}, nil
}

// Resolve maps an identity returned by an external provider to a local user and issues a session token pair.
func (m *SSOManager) Resolve(ctx context.Context, identity providers.Identity, opts ResolveOptions) (TokenPair, *models.User, *models.Session, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	email := strings.TrimSpace(identity.Email)
	if email == "" {
		return TokenPair{}, nil, nil, ErrSSOEmailRequired
	}

	provider := normaliseProvider(identity.Provider)
	if provider == "" {
		provider = strings.TrimSpace(identity.Provider)
	}

	user, err := m.LinkIdentity(ctx, identity, opts.AutoProvision)
	if err != nil {
		return TokenPair{}, nil, nil, err
	}
	if !user.IsActive {
		return TokenPair{}, nil, nil, ErrSSOUserDisabled
	}

	meta := opts.SessionMeta

	subjectClaims := make(map[string]any)
	for k, v := range identity.RawClaims {
		if k != "" {
			subjectClaims[k] = v
		}
	}
	if len(identity.Groups) > 0 {
		subjectClaims["sso_groups"] = identity.Groups
	}

	tokens, session, err := m.sessions.CreateForSubject(AuthSubject{
		UserID:     user.ID,
		Provider:   provider,
		ExternalID: identity.Subject,
		Email:      email,
		Claims:     subjectClaims,
	}, meta)
	if err != nil {
		return TokenPair{}, nil, nil, fmt.Errorf("sso manager: create session: %w", err)
	}

	now := m.clock()
	lastIP := strings.TrimSpace(meta.IPAddress)
	update := map[string]any{
		"last_login_at": now,
		"last_login_ip": lastIP,
	}
	if err := m.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).Updates(update).Error; err == nil {
		user.LastLoginAt = &now
		user.LastLoginIP = lastIP
	}

	return tokens, user, session, nil
}

// LinkIdentity associates an external identity with a user account, optionally provisioning new users.
func (m *SSOManager) LinkIdentity(ctx context.Context, identity providers.Identity, autoProvision bool) (*models.User, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	email := strings.TrimSpace(identity.Email)
	if email == "" {
		return nil, ErrSSOEmailRequired
	}

	return m.findOrProvisionUser(ctx, identity, strings.ToLower(email), autoProvision)
}

func (m *SSOManager) findOrProvisionUser(ctx context.Context, identity providers.Identity, email string, autoProvision bool) (*models.User, error) {
	incomingProvider := normaliseProvider(identity.Provider)
	subject := strings.TrimSpace(identity.Subject)

	var user models.User
	err := m.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).Take(&user).Error
	switch {
	case err == nil:
		existingProvider := normaliseProvider(user.AuthProvider)
		if existingProvider == "" {
			existingProvider = "local"
		}
		if existingProvider != "" && existingProvider != incomingProvider {
			return nil, ErrSSOUserNotFound
		}

		updates := map[string]any{}
		if user.AuthProvider != incomingProvider {
			updates["auth_provider"] = incomingProvider
		}
		if subject != "" && strings.TrimSpace(user.AuthSubject) != subject {
			updates["auth_subject"] = subject
		}

		if firstName := strings.TrimSpace(identity.FirstName); firstName != "" && firstName != user.FirstName {
			updates["first_name"] = firstName
		}
		if lastName := strings.TrimSpace(identity.LastName); lastName != "" && lastName != user.LastName {
			updates["last_name"] = lastName
		}

		if len(updates) > 0 {
			if err := m.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
				return nil, fmt.Errorf("sso manager: update user: %w", err)
			}
			if err := m.db.WithContext(ctx).Take(&user, "id = ?", user.ID).Error; err != nil {
				return nil, fmt.Errorf("sso manager: reload user: %w", err)
			}
		}

		return &user, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		if !autoProvision {
			return nil, ErrSSOUserNotFound
		}
		return m.provisionUser(ctx, identity, email)
	default:
		return nil, fmt.Errorf("sso manager: find user: %w", err)
	}
}

func (m *SSOManager) provisionUser(ctx context.Context, identity providers.Identity, email string) (*models.User, error) {
	if strings.TrimSpace(email) == "" {
		return nil, ErrSSOEmailRequired
	}

	provider := normaliseProvider(identity.Provider)
	if provider == "" {
		provider = "external"
	}
	subject := strings.TrimSpace(identity.Subject)

	var created *models.User
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hashedPassword, err := m.generatePlaceholderPassword()
		if err != nil {
			return err
		}

		baseUsername := m.deriveUsername(identity, email)

		for attempt := 0; attempt < 50; attempt++ {
			username := baseUsername
			if attempt > 0 {
				username = fmt.Sprintf("%s-%d", baseUsername, attempt)
			}

			user := &models.User{
				Username:     username,
				Email:        strings.ToLower(email),
				Password:     hashedPassword,
				FirstName:    strings.TrimSpace(identity.FirstName),
				LastName:     strings.TrimSpace(identity.LastName),
				IsActive:     true,
				AuthProvider: provider,
				AuthSubject:  subject,
			}

			if err := tx.Create(user).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) || isUniqueConstraintError(err) {
					continue
				}
				return fmt.Errorf("sso manager: create user: %w", err)
			}

			created = user
			return nil
		}

		return errors.New("sso manager: unable to generate unique username")
	})

	if err != nil {
		return nil, err
	}

	return created, nil
}

func (m *SSOManager) deriveUsername(identity providers.Identity, email string) string {
	if raw, ok := identity.RawClaims["username"]; ok {
		if candidate := claimFirstString(raw); candidate != "" {
			if sanitised := sanitiseUsername(candidate); sanitised != "" {
				return sanitised
			}
		}
	}

	parts := strings.Split(email, "@")
	base := strings.TrimSpace(parts[0])
	if base == "" {
		base = fmt.Sprintf("%s-%s", strings.TrimSpace(identity.Provider), strings.TrimSpace(identity.Subject))
	}
	sanitised := sanitiseUsername(base)
	if sanitised == "" {
		return "user"
	}
	return sanitised
}

func normaliseProvider(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func claimFirstString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		if len(v) == 0 {
			return ""
		}
		return strings.TrimSpace(v[0])
	case []any:
		for _, item := range v {
			if s := claimFirstString(item); s != "" {
				return s
			}
		}
	}
	return ""
}

func (m *SSOManager) generatePlaceholderPassword() (string, error) {
	token, err := crypto.GenerateToken(48)
	if err != nil {
		return "", fmt.Errorf("sso manager: generate placeholder password: %w", err)
	}
	hashed, err := crypto.HashPassword(token)
	if err != nil {
		return "", fmt.Errorf("sso manager: hash placeholder password: %w", err)
	}
	return hashed, nil
}

func sanitiseUsername(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	var b strings.Builder
	var lastHyphen bool
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteRune('-')
				lastHyphen = true
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	return result
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
