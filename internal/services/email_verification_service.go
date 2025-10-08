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
	defaultVerificationExpiry     = 24 * time.Hour
	defaultVerificationTokenBytes = 48
)

var (
	// ErrVerificationNotFound indicates the token does not exist.
	ErrVerificationNotFound = errors.New("email verification: not found")
	// ErrVerificationExpired indicates the verification token has expired.
	ErrVerificationExpired = errors.New("email verification: expired")
	// ErrVerificationUsed signals that the verification token has already been consumed.
	ErrVerificationUsed = errors.New("email verification: already used")
)

// VerificationOption customises the EmailVerificationService.
type VerificationOption func(*EmailVerificationService)

// WithVerificationBaseURL sets the base URL used in verification links.
func WithVerificationBaseURL(url string) VerificationOption {
	return func(s *EmailVerificationService) {
		s.baseURL = strings.TrimRight(url, "/")
	}
}

// WithVerificationExpiry overrides the token lifetime.
func WithVerificationExpiry(d time.Duration) VerificationOption {
	return func(s *EmailVerificationService) {
		if d > 0 {
			s.expiry = d
		}
	}
}

// WithVerificationTokenSize adjusts the number of random bytes in generated tokens.
func WithVerificationTokenSize(size int) VerificationOption {
	return func(s *EmailVerificationService) {
		if size > 0 {
			s.tokenLength = size
		}
	}
}

// WithVerificationClock injects a custom time source.
func WithVerificationClock(clock func() time.Time) VerificationOption {
	return func(s *EmailVerificationService) {
		if clock != nil {
			s.now = clock
		}
	}
}

// EmailVerificationService manages email verification tokens for local registrations.
type EmailVerificationService struct {
	db          *gorm.DB
	mailer      mail.Mailer
	baseURL     string
	expiry      time.Duration
	tokenLength int
	now         func() time.Time
}

// NewEmailVerificationService constructs a verification service with the provided dependencies.
func NewEmailVerificationService(db *gorm.DB, mailer mail.Mailer, opts ...VerificationOption) (*EmailVerificationService, error) {
	if db == nil {
		return nil, errors.New("email verification service: db is required")
	}

	service := &EmailVerificationService{
		db:          db,
		mailer:      mailer,
		expiry:      defaultVerificationExpiry,
		tokenLength: defaultVerificationTokenBytes,
		now:         time.Now,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

// CreateToken issues a verification token for the given user and dispatches an email when a mailer is configured.
func (s *EmailVerificationService) CreateToken(ctx context.Context, userID, email string) (string, string, error) {
	userID = strings.TrimSpace(userID)
	email = strings.TrimSpace(strings.ToLower(email))
	if userID == "" {
		return "", "", errors.New("email verification service: user id is required")
	}
	if email == "" {
		return "", "", errors.New("email verification service: email is required")
	}

	token, err := crypto.GenerateToken(s.tokenLength)
	if err != nil {
		return "", "", fmt.Errorf("email verification service: generate token: %w", err)
	}

	now := s.now()
	verification := models.EmailVerification{
		UserID:    userID,
		TokenHash: verificationHash(token),
		ExpiresAt: now.Add(s.expiry),
	}

	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.EmailVerification{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", fmt.Errorf("email verification service: cleanup existing: %w", err)
	}

	if err := s.db.WithContext(ctx).Create(&verification).Error; err != nil {
		return "", "", fmt.Errorf("email verification service: create token: %w", err)
	}

	link := s.verificationLink(token)

	if s.mailer != nil {
		message := mail.Message{
			To:      []string{email},
			Subject: "Confirm your ShellCN account",
			Body:    s.verificationBody(link),
		}
		if mailErr := s.mailer.Send(ctx, message); mailErr != nil && !errors.Is(mailErr, mail.ErrSMTPDisabled) {
			return "", "", fmt.Errorf("email verification service: send email: %w", mailErr)
		}
	}

	return token, link, nil
}

// VerifyToken validates and consumes a verification token.
func (s *EmailVerificationService) VerifyToken(ctx context.Context, token string) (*models.EmailVerification, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("email verification service: token is required")
	}

	var verification models.EmailVerification
	if err := s.db.WithContext(ctx).
		Where("token_hash = ?", verificationHash(token)).
		First(&verification).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVerificationNotFound
		}
		return nil, fmt.Errorf("email verification service: find token: %w", err)
	}

	now := s.now()
	if verification.ExpiresAt.Before(now) {
		return nil, ErrVerificationExpired
	}
	if verification.VerifiedAt != nil {
		return nil, ErrVerificationUsed
	}

	if err := s.db.WithContext(ctx).
		Model(&verification).
		Updates(map[string]any{"verified_at": now}).Error; err != nil {
		return nil, fmt.Errorf("email verification service: mark verified: %w", err)
	}

	verification.VerifiedAt = &now
	return &verification, nil
}

func (s *EmailVerificationService) verificationLink(token string) string {
	if s.baseURL == "" {
		return token
	}
	return fmt.Sprintf("%s?token=%s", s.baseURL, token)
}

func (s *EmailVerificationService) verificationBody(link string) string {
	return fmt.Sprintf("Welcome to ShellCN!\n\nPlease confirm your email address by visiting the link below:\n%s\n\nIf you did not create an account, you can ignore this message.\n", link)
}

func verificationHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
