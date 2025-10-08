package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
	"github.com/charlesng35/shellcn/pkg/mail"
)

const (
	defaultInviteExpiry     = 72 * time.Hour
	defaultInviteTokenBytes = 48
)

var (
	// ErrInviteNotFound indicates no invite matches the provided token.
	ErrInviteNotFound = errors.New("invite: not found")
	// ErrInviteExpired indicates the invite token has expired.
	ErrInviteExpired = errors.New("invite: expired")
	// ErrInviteAlreadyUsed signals that the invite has already been accepted.
	ErrInviteAlreadyUsed = errors.New("invite: already accepted")
)

// InviteOption customises InviteService behaviour.
type InviteOption func(*InviteService)

// WithInviteBaseURL configures the base URL used to create invite hyperlinks.
func WithInviteBaseURL(url string) InviteOption {
	return func(s *InviteService) {
		s.baseURL = strings.TrimRight(url, "/")
	}
}

// WithInviteExpiry overrides the invite token lifetime.
func WithInviteExpiry(d time.Duration) InviteOption {
	return func(s *InviteService) {
		if d > 0 {
			s.expiry = d
		}
	}
}

// WithInviteTokenSize adjusts the random token length in bytes.
func WithInviteTokenSize(size int) InviteOption {
	return func(s *InviteService) {
		if size > 0 {
			s.tokenLength = size
		}
	}
}

// WithInviteClock injects a custom clock primarily for testing.
func WithInviteClock(clock func() time.Time) InviteOption {
	return func(s *InviteService) {
		if clock != nil {
			s.now = clock
		}
	}
}

// InviteService manages generation and consumption of user invite tokens.
type InviteService struct {
	db          *gorm.DB
	mailer      mail.Mailer
	baseURL     string
	expiry      time.Duration
	tokenLength int
	now         func() time.Time
}

// NewInviteService constructs an InviteService with the provided dependencies.
func NewInviteService(db *gorm.DB, mailer mail.Mailer, opts ...InviteOption) (*InviteService, error) {
	if db == nil {
		return nil, errors.New("invite service: db is required")
	}

	service := &InviteService{
		db:          db,
		mailer:      mailer,
		expiry:      defaultInviteExpiry,
		tokenLength: defaultInviteTokenBytes,
		now:         time.Now,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

// GenerateInvite creates a new invite token for the provided email address and optionally dispatches an email.
func (s *InviteService) GenerateInvite(ctx context.Context, email, invitedBy string) (token, link string, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", "", errors.New("invite service: email is required")
	}

	rawToken, err := crypto.GenerateToken(s.tokenLength)
	if err != nil {
		return "", "", fmt.Errorf("invite service: generate token: %w", err)
	}

	now := s.now()
	invite := models.UserInvite{
		Email:     email,
		TokenHash: tokenHash(rawToken),
		InvitedBy: strings.TrimSpace(invitedBy),
		ExpiresAt: now.Add(s.expiry),
	}

	if err := s.db.WithContext(ctx).Create(&invite).Error; err != nil {
		return "", "", fmt.Errorf("invite service: create invite: %w", err)
	}

	link = s.inviteLink(rawToken)

	if s.mailer != nil {
		message := mail.Message{
			To:      []string{email},
			Subject: "You're invited to ShellCN",
			Body:    s.inviteBody(link, email),
		}
		if mailErr := s.mailer.Send(ctx, message); mailErr != nil && !errors.Is(mailErr, mail.ErrSMTPDisabled) {
			return "", "", fmt.Errorf("invite service: send email: %w", mailErr)
		}
	}

	return rawToken, link, nil
}

// RedeemInvite validates the token and marks the invite as accepted.
func (s *InviteService) RedeemInvite(ctx context.Context, token string) (*models.UserInvite, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("invite service: token is required")
	}

	var invite models.UserInvite
	if err := s.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash(token)).
		First(&invite).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInviteNotFound
		}
		return nil, fmt.Errorf("invite service: find invite: %w", err)
	}

	now := s.now()
	if invite.ExpiresAt.Before(now) {
		return nil, ErrInviteExpired
	}
	if invite.AcceptedAt != nil {
		return nil, ErrInviteAlreadyUsed
	}

	if err := s.db.WithContext(ctx).
		Model(&invite).
		Updates(map[string]any{"accepted_at": now}).Error; err != nil {
		return nil, fmt.Errorf("invite service: mark accepted: %w", err)
	}

	invite.AcceptedAt = &now
	return &invite, nil
}

func (s *InviteService) inviteLink(token string) string {
	if s.baseURL == "" {
		return token
	}
	return fmt.Sprintf("%s?token=%s", s.baseURL, token)
}

func (s *InviteService) inviteBody(link, email string) string {
	return fmt.Sprintf("Hello,\n\nYou have been invited to join ShellCN. Use the following link to accept your invite:\n%s\n\nIf you did not expect this email, you can ignore it.\n", link)
}

func tokenHash(token string) string {
	checksum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(checksum[:])
}
