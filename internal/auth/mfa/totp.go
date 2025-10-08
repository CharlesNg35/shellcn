package mfa

import (
	cryptoRand "crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

const (
	defaultIssuer          = "ShellCN"
	defaultBackupCodeCount = 10
	defaultQRCodeSize      = 256
)

// Option allows customising the TOTP service.
type Option func(*TOTPService)

// WithIssuer overrides the default issuer string encoded in provisioning URIs.
func WithIssuer(issuer string) Option {
	return func(s *TOTPService) {
		if strings.TrimSpace(issuer) != "" {
			s.issuer = issuer
		}
	}
}

// WithBackupCodeCount overrides the number of backup codes generated for users.
func WithBackupCodeCount(count int) Option {
	return func(s *TOTPService) {
		if count > 0 {
			s.backupCodes = count
		}
	}
}

// WithQRCodeSize controls the pixel size of generated QR codes.
func WithQRCodeSize(size int) Option {
	return func(s *TOTPService) {
		if size > 0 {
			s.qrCodeSize = size
		}
	}
}

// WithClock injects a custom clock, primarily for testing.
func WithClock(clock func() time.Time) Option {
	return func(s *TOTPService) {
		if clock != nil {
			s.now = clock
		}
	}
}

// TOTPService manages user MFA secrets, backup codes, and QR provisioning.
type TOTPService struct {
	db            *gorm.DB
	encryptionKey []byte

	issuer      string
	backupCodes int
	qrCodeSize  int
	now         func() time.Time
}

// NewTOTPService constructs a TOTP service backed by the provided database.
func NewTOTPService(db *gorm.DB, encryptionKey []byte, opts ...Option) (*TOTPService, error) {
	if db == nil {
		return nil, errors.New("totp: db is required")
	}
	if len(encryptionKey) == 0 {
		return nil, errors.New("totp: encryption key is required")
	}

	service := &TOTPService{
		db:            db,
		encryptionKey: encryptionKey,
		issuer:        defaultIssuer,
		backupCodes:   defaultBackupCodeCount,
		qrCodeSize:    defaultQRCodeSize,
		now:           time.Now,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

// GenerateSecret provisions a new MFA secret and backup codes for a user.
func (s *TOTPService) GenerateSecret(userID, username string) (*otp.Key, []string, error) {
	userID = strings.TrimSpace(userID)
	username = strings.TrimSpace(username)

	if userID == "" || username == "" {
		return nil, nil, errors.New("totp: user id and username are required")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: username,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("totp: generate key: %w", err)
	}

	backupCodes := make([]string, s.backupCodes)
	for i := range backupCodes {
		code, err := generateBackupCode()
		if err != nil {
			return nil, nil, fmt.Errorf("totp: generate backup code: %w", err)
		}
		backupCodes[i] = code
	}

	encryptedSecret, err := crypto.Encrypt([]byte(key.Secret()), s.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("totp: encrypt secret: %w", err)
	}

	hashedCodes := make([]string, len(backupCodes))
	for i, code := range backupCodes {
		hash, err := crypto.HashPassword(code)
		if err != nil {
			return nil, nil, fmt.Errorf("totp: hash backup code: %w", err)
		}
		hashedCodes[i] = hash
	}

	codesJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		return nil, nil, fmt.Errorf("totp: marshal backup codes: %w", err)
	}

	var secret models.MFASecret
	if err := s.db.Where("user_id = ?", userID).First(&secret).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, fmt.Errorf("totp: load mfa secret: %w", err)
		}

		secret = models.MFASecret{
			UserID:      userID,
			Secret:      encryptedSecret,
			BackupCodes: string(codesJSON),
		}

		if err := s.db.Create(&secret).Error; err != nil {
			return nil, nil, fmt.Errorf("totp: create mfa secret: %w", err)
		}
	} else {
		secret.Secret = encryptedSecret
		secret.BackupCodes = string(codesJSON)
		secret.LastUsedAt = nil

		if err := s.db.Save(&secret).Error; err != nil {
			return nil, nil, fmt.Errorf("totp: update mfa secret: %w", err)
		}
	}

	return key, backupCodes, nil
}

// VerifyCode checks a submitted TOTP code against the stored secret.
func (s *TOTPService) VerifyCode(userID, code string) (bool, error) {
	userID = strings.TrimSpace(userID)
	code = strings.TrimSpace(code)
	if userID == "" || code == "" {
		return false, errors.New("totp: user id and code are required")
	}

	secret, err := s.loadSecret(userID)
	if err != nil {
		return false, err
	}

	rawSecret, err := crypto.Decrypt(secret.Secret, s.encryptionKey)
	if err != nil {
		return false, fmt.Errorf("totp: decrypt secret: %w", err)
	}

	valid := totp.Validate(code, string(rawSecret))
	if valid {
		now := s.now()
		if err := s.db.Model(secret).Update("last_used_at", &now).Error; err != nil {
			return false, fmt.Errorf("totp: update last used: %w", err)
		}
	}

	return valid, nil
}

// UseBackupCode validates and consumes a single backup code.
func (s *TOTPService) UseBackupCode(userID, code string) (bool, error) {
	userID = strings.TrimSpace(userID)
	code = strings.TrimSpace(code)
	if userID == "" || code == "" {
		return false, errors.New("totp: user id and code are required")
	}

	secret, err := s.loadSecret(userID)
	if err != nil {
		return false, err
	}

	var hashedCodes []string
	if err := json.Unmarshal([]byte(secret.BackupCodes), &hashedCodes); err != nil {
		return false, fmt.Errorf("totp: unmarshal backup codes: %w", err)
	}

	consumed := false
	for i, stored := range hashedCodes {
		if crypto.VerifyPassword(stored, code) {
			hashedCodes = append(hashedCodes[:i], hashedCodes[i+1:]...)
			consumed = true
			break
		}
	}

	if !consumed {
		return false, nil
	}

	encoded, err := json.Marshal(hashedCodes)
	if err != nil {
		return false, fmt.Errorf("totp: marshal backup codes: %w", err)
	}

	if err := s.db.Model(secret).Update("backup_codes", string(encoded)).Error; err != nil {
		return false, fmt.Errorf("totp: update backup codes: %w", err)
	}

	return true, nil
}

// RemainingBackupCodes returns the number of backup codes still available.
func (s *TOTPService) RemainingBackupCodes(userID string) (int, error) {
	secret, err := s.loadSecret(strings.TrimSpace(userID))
	if err != nil {
		return 0, err
	}

	var hashedCodes []string
	if err := json.Unmarshal([]byte(secret.BackupCodes), &hashedCodes); err != nil {
		return 0, fmt.Errorf("totp: unmarshal backup codes: %w", err)
	}

	return len(hashedCodes), nil
}

// GenerateQRCode returns a PNG-encoded QR code for the provided TOTP key.
func (s *TOTPService) GenerateQRCode(key *otp.Key) ([]byte, error) {
	if key == nil {
		return nil, errors.New("totp: key is required")
	}
	return qrcode.Encode(key.String(), qrcode.Medium, s.qrCodeSize)
}

func (s *TOTPService) loadSecret(userID string) (*models.MFASecret, error) {
	if userID == "" {
		return nil, errors.New("totp: user id is required")
	}

	var secret models.MFASecret
	if err := s.db.Where("user_id = ?", userID).First(&secret).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("totp: secret not found for user %s", userID)
		}
		return nil, fmt.Errorf("totp: load secret: %w", err)
	}

	return &secret, nil
}

func generateBackupCode() (string, error) {
	buf := make([]byte, 5)
	if _, err := cryptoRand.Read(buf); err != nil {
		return "", err
	}

	return base32.StdEncoding.EncodeToString(buf)[:8], nil
}
