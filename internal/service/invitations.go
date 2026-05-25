package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/store"
)

// DefaultInvitationTTL is how long an unaccepted invitation stays valid.
const DefaultInvitationTTL = 72 * time.Hour

// ErrInvitationInvalid is returned for an unknown, expired, or consumed token.
var ErrInvitationInvalid = errors.New("service: invalid or expired invitation")

// Mailer sends invitation emails; satisfied by internal/email.Mailer.
type Mailer interface {
	Enabled() bool
	Send(to, subject, body string) error
}

// InvitationService issues account invitations, sends the link when email is
// configured, and consumes a token to create the account on acceptance.
type InvitationService struct {
	invites store.InvitationStore
	users   *UserService
	mailer  Mailer
	ttl     time.Duration
	now     func() time.Time
}

func NewInvitationService(invites store.InvitationStore, users *UserService, mailer Mailer) *InvitationService {
	return &InvitationService{invites: invites, users: users, mailer: mailer, ttl: DefaultInvitationTTL, now: time.Now}
}

// EmailEnabled reports whether invitations are also delivered by email.
func (s *InvitationService) EmailEnabled() bool {
	return s.mailer != nil && s.mailer.Enabled()
}

// Create issues an invitation. It returns the stored record and the raw token,
// which appears only here (in the acceptURL link) and as a stored hash.
func (s *InvitationService) Create(ctx context.Context, email string, role models.Role, inviterID, acceptURL string) (models.Invitation, string, bool, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return models.Invitation{}, "", false, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	now := s.now()
	inv := models.Invitation{
		ID:        uuid.NewString(),
		Email:     email,
		Role:      role,
		TokenHash: hashToken(token),
		Status:    models.InvitePending,
		InvitedBy: inviterID,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}
	if err := s.invites.Create(ctx, &inv); err != nil {
		return models.Invitation{}, "", false, err
	}

	emailSent := false
	if s.mailer != nil && s.mailer.Enabled() {
		body := fmt.Sprintf("You've been invited to ShellCN.\n\nAccept and set your password:\n%s%s\n\nThis link expires %s.",
			acceptURL, token, inv.ExpiresAt.Format(time.RFC1123))
		emailSent = s.mailer.Send(email, "You're invited to ShellCN", body) == nil
	}
	return inv, token, emailSent, nil
}

func (s *InvitationService) List(ctx context.Context) ([]models.InvitationSummary, error) {
	list, err := s.invites.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.InvitationSummary, 0, len(list))
	for _, i := range list {
		out = append(out, i.Summary())
	}
	return out, nil
}

func (s *InvitationService) Revoke(ctx context.Context, id string) error {
	inv, err := s.invites.Get(ctx, id)
	if err != nil {
		return err
	}
	inv.Status = models.InviteRevoked
	return s.invites.Update(ctx, &inv)
}

// Lookup validates a raw token and returns the pending invitation behind it.
func (s *InvitationService) Lookup(ctx context.Context, token string) (models.Invitation, error) {
	inv, err := s.invites.GetByTokenHash(ctx, hashToken(token))
	if err != nil {
		return models.Invitation{}, ErrInvitationInvalid
	}
	if inv.Status != models.InvitePending || s.now().After(inv.ExpiresAt) {
		return models.Invitation{}, ErrInvitationInvalid
	}
	return inv, nil
}

// Accept consumes an invitation, creating the account under the chosen username
// with the invitation's email and role.
func (s *InvitationService) Accept(ctx context.Context, token, username, password string) (models.User, error) {
	inv, err := s.Lookup(ctx, token)
	if err != nil {
		return models.User{}, err
	}
	now := s.now()
	consumed, err := s.invites.Consume(ctx, inv.ID, now)
	if err != nil {
		return models.User{}, err
	}
	if !consumed {
		return models.User{}, ErrInvitationInvalid
	}
	user, err := s.users.Create(ctx, NewUserInput{
		Username: username,
		Email:    inv.Email,
		Roles:    []models.Role{inv.Role},
		Password: password,
	})
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}
