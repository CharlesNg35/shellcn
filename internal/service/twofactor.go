package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/store"
)

var (
	// ErrInvalidCode is returned for a wrong TOTP or recovery code.
	ErrInvalidCode = errors.New("twofactor: invalid code")
	// ErrTOTPNotEnrolled is returned when a confirm/verify is attempted with no secret.
	ErrTOTPNotEnrolled = errors.New("twofactor: no enrollment in progress")
	// ErrTOTPNotEnabled is returned when an action requires active 2FA.
	ErrTOTPNotEnabled = errors.New("twofactor: not enabled")
)

const (
	recoveryCodeCount = 10
	// remindInterval is how long after a dismissed nudge we ask again.
	remindInterval = 72 * time.Hour
)

// TOTPEnrollment carries everything the client needs to add the account to an
// authenticator app: the otpauth URL (encoded as a QR), the raw base32 secret
// for manual entry, and a ready-to-render QR data URL.
type TOTPEnrollment struct {
	Secret      string
	OTPAuthURL  string
	QRDataURL   string
	AccountName string
}

// TwoFactorService owns TOTP enrollment, verification, and recovery codes. The
// shared secret is encrypted at rest by the vault; only hashes of recovery codes
// are stored.
type TwoFactorService struct {
	users  store.UserStore
	vault  secrets.SecretStore
	issuer string
	now    func() time.Time
}

func NewTwoFactorService(users store.UserStore, vault secrets.SecretStore, issuer string) *TwoFactorService {
	if issuer == "" {
		issuer = "ShellCN"
	}
	return &TwoFactorService{users: users, vault: vault, issuer: issuer, now: time.Now}
}

// BeginEnrollment generates a fresh secret, stores it encrypted as a pending
// (not-yet-confirmed) enrollment, and returns the provisioning material.
func (s *TwoFactorService) BeginEnrollment(ctx context.Context, user models.User) (TOTPEnrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: s.issuer, AccountName: user.Username})
	if err != nil {
		return TOTPEnrollment{}, err
	}
	enc, err := s.vault.Encrypt(ctx, []byte(key.Secret()))
	if err != nil {
		return TOTPEnrollment{}, err
	}
	if err := s.users.SetTwoFactor(ctx, user.ID, enc, false, nil); err != nil {
		return TOTPEnrollment{}, err
	}
	qr, err := qrDataURL(key.URL())
	if err != nil {
		return TOTPEnrollment{}, err
	}
	return TOTPEnrollment{Secret: key.Secret(), OTPAuthURL: key.URL(), QRDataURL: qr, AccountName: user.Username}, nil
}

// ConfirmEnrollment validates the first code against the pending secret, marks
// 2FA enabled, and returns one-time recovery codes (shown to the user once).
func (s *TwoFactorService) ConfirmEnrollment(ctx context.Context, user models.User, code string) ([]string, error) {
	secret, err := s.decryptSecret(ctx, user)
	if err != nil {
		return nil, err
	}
	if !totp.Validate(strings.TrimSpace(code), secret) {
		return nil, ErrInvalidCode
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	if err := s.users.SetTwoFactor(ctx, user.ID, user.TOTPSecret, true, hashes); err != nil {
		return nil, err
	}
	return codes, nil
}

// Disable turns off 2FA after verifying a current code (TOTP or recovery),
// clearing the secret and recovery codes.
func (s *TwoFactorService) Disable(ctx context.Context, user models.User, code string) error {
	if !user.TOTPEnabled {
		return ErrTOTPNotEnabled
	}
	ok, err := s.Verify(ctx, user, code)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCode
	}
	return s.users.SetTwoFactor(ctx, user.ID, nil, false, nil)
}

// RegenerateRecoveryCodes issues a fresh set after verifying a current code,
// invalidating the previous codes.
func (s *TwoFactorService) RegenerateRecoveryCodes(ctx context.Context, user models.User, code string) ([]string, error) {
	if !user.TOTPEnabled {
		return nil, ErrTOTPNotEnabled
	}
	ok, err := s.Verify(ctx, user, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCode
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	if err := s.users.SetTwoFactor(ctx, user.ID, user.TOTPSecret, true, hashes); err != nil {
		return nil, err
	}
	return codes, nil
}

// Verify checks a login code against the user's TOTP secret, falling back to a
// single-use recovery code (which it then consumes). It is the gate between
// password success and session creation.
func (s *TwoFactorService) Verify(ctx context.Context, user models.User, code string) (bool, error) {
	if !user.TOTPEnabled {
		return false, ErrTOTPNotEnabled
	}
	code = strings.TrimSpace(code)
	secret, err := s.decryptSecret(ctx, user)
	if err != nil {
		return false, err
	}
	if totp.Validate(code, secret) {
		return true, nil
	}
	return s.consumeRecoveryCode(ctx, user, code)
}

func (s *TwoFactorService) consumeRecoveryCode(ctx context.Context, user models.User, code string) (bool, error) {
	want := hashRecoveryCode(code)
	remaining := make([]string, 0, len(user.RecoveryCodeHashes))
	matched := false
	for _, h := range user.RecoveryCodeHashes {
		if !matched && subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 {
			matched = true
			continue
		}
		remaining = append(remaining, h)
	}
	if !matched {
		return false, nil
	}
	if err := s.users.SetTwoFactor(ctx, user.ID, user.TOTPSecret, true, remaining); err != nil {
		return false, err
	}
	return true, nil
}

// ShouldRemind reports whether to nudge the user to enable 2FA: only when it is
// off and they have never been reminded or the reminder interval has elapsed.
func (s *TwoFactorService) ShouldRemind(user models.User) bool {
	if user.TOTPEnabled {
		return false
	}
	return user.MFARemindedAt == nil || s.now().Sub(*user.MFARemindedAt) >= remindInterval
}

// Remind records that the user was just nudged, so the next prompt waits out the
// reminder interval.
func (s *TwoFactorService) Remind(ctx context.Context, userID string) error {
	at := s.now()
	return s.users.SetMFARemindedAt(ctx, userID, &at)
}

func (s *TwoFactorService) decryptSecret(ctx context.Context, user models.User) (string, error) {
	if len(user.TOTPSecret) == 0 {
		return "", ErrTOTPNotEnrolled
	}
	plain, err := s.vault.Decrypt(ctx, user.TOTPSecret)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

var recoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// newRecoveryCodes returns the plaintext codes (shown once) and their hashes
// (stored). Codes are high-entropy, so a single SHA-256 is sufficient.
func newRecoveryCodes() (codes, hashes []string, err error) {
	codes = make([]string, recoveryCodeCount)
	hashes = make([]string, recoveryCodeCount)
	for i := range codes {
		buf := make([]byte, 10)
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, err
		}
		raw := strings.ToLower(recoveryEncoding.EncodeToString(buf))
		code := fmt.Sprintf("%s-%s", raw[:4], raw[4:8])
		codes[i] = code
		hashes[i] = hashRecoveryCode(code)
	}
	return codes, hashes, nil
}

func hashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

// qrDataURL renders an otpauth URL as a base64-encoded PNG data URL, so the
// client can show the QR without shipping a QR library or the raw secret in the DOM.
func qrDataURL(otpauthURL string) (string, error) {
	key, err := otp.NewKeyFromURL(otpauthURL)
	if err != nil {
		return "", err
	}
	img, err := key.Image(220, 220)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
