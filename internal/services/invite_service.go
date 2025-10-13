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
	// ErrInviteAlreadyPending indicates an invite already exists for the email.
	ErrInviteAlreadyPending = errors.New("invite: already pending")
	// ErrInviteEmailInUse indicates an existing user account already uses the email.
	ErrInviteEmailInUse = errors.New("invite: email already registered")
	// ErrInviteUserAlreadyInTeam indicates the user is already a member of the target team.
	ErrInviteUserAlreadyInTeam = errors.New("invite: user already in team")
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

// WithInviteAuditService attaches an AuditService used to record invite lifecycle events.
func WithInviteAuditService(audit *AuditService) InviteOption {
	return func(s *InviteService) {
		s.audit = audit
	}
}

// InviteService manages generation and consumption of user invite tokens.
type InviteService struct {
	db          *gorm.DB
	mailer      mail.Mailer
	audit       *AuditService
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
		audit:       nil,
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
func (s *InviteService) GenerateInvite(ctx context.Context, email, invitedBy, teamID string) (invite *models.UserInvite, token, link string, err error) {
	ctx = ensureContext(ctx)

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, "", "", errors.New("invite service: email is required")
	}
	teamID = strings.TrimSpace(teamID)

	now := s.now()

	var existing models.UserInvite
	if err := s.db.WithContext(ctx).
		Where("LOWER(email) = ? AND accepted_at IS NULL AND expires_at > ?", email, now).
		Take(&existing).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", "", fmt.Errorf("invite service: check existing invites: %w", err)
	} else if err == nil {
		return nil, "", "", ErrInviteAlreadyPending
	}

	existingUser, err := s.findUserByEmail(ctx, email)
	if err != nil {
		return nil, "", "", err
	}

	var team *models.Team
	if teamID != "" {
		var teamModel models.Team
		if err := s.db.WithContext(ctx).First(&teamModel, "id = ?", teamID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", "", ErrTeamNotFound
			}
			return nil, "", "", fmt.Errorf("invite service: verify team: %w", err)
		}
		team = &teamModel

		if existingUser != nil {
			inTeam, err := s.userAlreadyInTeam(ctx, existingUser.ID, team.ID)
			if err != nil {
				return nil, "", "", err
			}
			if inTeam {
				return nil, "", "", ErrInviteUserAlreadyInTeam
			}
		}
	} else if existingUser != nil {
		return nil, "", "", ErrInviteEmailInUse
	}

	rawToken, err := crypto.GenerateToken(s.tokenLength)
	if err != nil {
		return nil, "", "", fmt.Errorf("invite service: generate token: %w", err)
	}

	invite = &models.UserInvite{
		Email:     email,
		TokenHash: tokenHash(rawToken),
		InvitedBy: strings.TrimSpace(invitedBy),
		ExpiresAt: now.Add(s.expiry),
	}
	if team != nil {
		invite.TeamID = &team.ID
	}

	if err := s.db.WithContext(ctx).Create(invite).Error; err != nil {
		return nil, "", "", fmt.Errorf("invite service: create invite: %w", err)
	}

	if team != nil {
		invite.Team = team
	}

	link = s.inviteLink(rawToken)

	metadata := map[string]any{
		"email":      invite.Email,
		"invited_by": strings.TrimSpace(invitedBy),
	}
	if invite.TeamID != nil {
		metadata["team_id"] = *invite.TeamID
	}

	recordAudit(s.audit, ctx, AuditEntry{
		Action:   "invite.create",
		Resource: invite.ID,
		Result:   "success",
		Metadata: metadata,
	})

	if s.mailer != nil {
		message := mail.Message{
			To:      []string{email},
			Subject: "You're invited to ShellCN",
			Body:    s.inviteBody(link, email),
		}
		if mailErr := s.mailer.Send(ctx, message); mailErr != nil && !errors.Is(mailErr, mail.ErrSMTPDisabled) {
			return nil, "", "", fmt.Errorf("invite service: send email: %w", mailErr)
		}
	}

	return invite, rawToken, link, nil
}

// RedeemInvite validates the token and marks the invite as accepted.
func (s *InviteService) RedeemInvite(ctx context.Context, token string) (*models.UserInvite, error) {
	ctx = ensureContext(ctx)

	invite, err := s.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if err := s.AcceptInvite(ctx, invite.ID); err != nil {
		return nil, err
	}

	invite.AcceptedAt = ptrTime(s.now())
	return invite, nil
}

// ValidateToken ensures the token corresponds to a pending invite that has not expired.
func (s *InviteService) ValidateToken(ctx context.Context, token string) (*models.UserInvite, error) {
	ctx = ensureContext(ctx)

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("invite service: token is required")
	}

	var invite models.UserInvite
	if err := s.db.WithContext(ctx).
		Preload("Team").
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

	return &invite, nil
}

// AcceptInvite marks a pending invite as accepted.
func (s *InviteService) AcceptInvite(ctx context.Context, inviteID string) error {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(inviteID) == "" {
		return errors.New("invite service: invite id is required")
	}

	now := s.now()

	result := s.db.WithContext(ctx).
		Model(&models.UserInvite{}).
		Where("id = ? AND accepted_at IS NULL", inviteID).
		Update("accepted_at", now)

	if result.Error != nil {
		return fmt.Errorf("invite service: mark accepted: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// Determine reason
		var invite models.UserInvite
		err := s.db.WithContext(ctx).First(&invite, "id = ?", inviteID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInviteNotFound
		}
		if err != nil {
			return fmt.Errorf("invite service: reload invite: %w", err)
		}
		if invite.AcceptedAt != nil {
			return ErrInviteAlreadyUsed
		}
		if invite.ExpiresAt.Before(now) {
			return ErrInviteExpired
		}
		return ErrInviteNotFound
	}

	recordAudit(s.audit, ctx, AuditEntry{
		Action:   "invite.accept",
		Resource: inviteID,
		Result:   "success",
	})

	return nil
}

// List returns invites, optionally filtered by status or search query.
func (s *InviteService) List(ctx context.Context, status, search string) ([]models.UserInvite, error) {
	ctx = ensureContext(ctx)

	query := s.db.WithContext(ctx).Model(&models.UserInvite{})
	now := s.now()

	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending":
		query = query.Where("accepted_at IS NULL AND expires_at > ?", now)
	case "expired":
		query = query.Where("accepted_at IS NULL AND expires_at <= ?", now)
	case "accepted":
		query = query.Where("accepted_at IS NOT NULL")
	}

	if trimmed := strings.ToLower(strings.TrimSpace(search)); trimmed != "" {
		like := "%" + trimmed + "%"
		query = query.Where("LOWER(email) LIKE ?", like)
	}

	var invites []models.UserInvite
	if err := query.
		Preload("Team").
		Order("created_at DESC").
		Find(&invites).Error; err != nil {
		return nil, fmt.Errorf("invite service: list invites: %w", err)
	}

	return invites, nil
}

// ResendInvite refreshes the invite token, optionally extends expiry, and dispatches an email notification.
func (s *InviteService) ResendInvite(ctx context.Context, inviteID string) (*models.UserInvite, string, string, error) {
	return s.refreshInvite(ctx, inviteID, true)
}

// IssueInviteLink refreshes the invite token and returns the new link without sending an email.
func (s *InviteService) IssueInviteLink(ctx context.Context, inviteID string) (*models.UserInvite, string, string, error) {
	return s.refreshInvite(ctx, inviteID, false)
}

func (s *InviteService) refreshInvite(ctx context.Context, inviteID string, sendEmail bool) (*models.UserInvite, string, string, error) {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(inviteID) == "" {
		return nil, "", "", errors.New("invite service: invite id is required")
	}

	var invite models.UserInvite
	if err := s.db.WithContext(ctx).
		Preload("Team").
		First(&invite, "id = ?", inviteID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", ErrInviteNotFound
		}
		return nil, "", "", fmt.Errorf("invite service: load invite: %w", err)
	}

	if invite.AcceptedAt != nil {
		return nil, "", "", ErrInviteAlreadyUsed
	}

	rawToken, err := crypto.GenerateToken(s.tokenLength)
	if err != nil {
		return nil, "", "", fmt.Errorf("invite service: generate token: %w", err)
	}

	now := s.now()
	newHash := tokenHash(rawToken)
	newExpiry := now.Add(s.expiry)

	if err := s.db.WithContext(ctx).Model(&invite).Updates(map[string]any{
		"token_hash": newHash,
		"expires_at": newExpiry,
	}).Error; err != nil {
		return nil, "", "", fmt.Errorf("invite service: refresh token: %w", err)
	}

	invite.TokenHash = newHash
	invite.ExpiresAt = newExpiry

	link := s.inviteLink(rawToken)

	action := "invite.link"
	if sendEmail && s.mailer != nil {
		message := mail.Message{
			To:      []string{invite.Email},
			Subject: "Your ShellCN invitation",
			Body:    s.inviteBody(link, invite.Email),
		}
		if mailErr := s.mailer.Send(ctx, message); mailErr != nil && !errors.Is(mailErr, mail.ErrSMTPDisabled) {
			return nil, "", "", fmt.Errorf("invite service: resend email: %w", mailErr)
		}
		action = "invite.resend"
	}

	recordAudit(s.audit, ctx, AuditEntry{
		Action:   action,
		Resource: invite.ID,
		Result:   "success",
		Metadata: map[string]any{
			"send_email": sendEmail && s.mailer != nil,
		},
	})

	return &invite, rawToken, link, nil
}

func (s *InviteService) findUserByEmail(ctx context.Context, email string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil
	}

	var user models.User
	err := s.db.WithContext(ctx).
		Preload("Teams").
		Where("LOWER(email) = ?", email).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("invite service: find user by email: %w", err)
	}
	return &user, nil
}

func (s *InviteService) userAlreadyInTeam(ctx context.Context, userID, teamID string) (bool, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(teamID) == "" {
		return false, nil
	}

	var count int64
	if err := s.db.WithContext(ctx).
		Table("user_teams").
		Where("user_id = ? AND team_id = ?", userID, teamID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("invite service: check team membership: %w", err)
	}
	return count > 0, nil
}

// Delete removes an invite if it has not been accepted already.
func (s *InviteService) Delete(ctx context.Context, inviteID string) error {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(inviteID) == "" {
		return errors.New("invite service: invite id is required")
	}

	var invite models.UserInvite
	if err := s.db.WithContext(ctx).First(&invite, "id = ?", inviteID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInviteNotFound
		}
		return fmt.Errorf("invite service: load invite: %w", err)
	}
	if invite.AcceptedAt != nil {
		return ErrInviteAlreadyUsed
	}

	if err := s.db.WithContext(ctx).Delete(&models.UserInvite{}, "id = ?", inviteID).Error; err != nil {
		return fmt.Errorf("invite service: delete invite: %w", err)
	}

	recordAudit(s.audit, ctx, AuditEntry{
		Action:   "invite.delete",
		Resource: invite.ID,
		Result:   "success",
		Metadata: map[string]any{
			"email": invite.Email,
		},
	})

	return nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

// RedeemInvite validates the token and marks the invite as accepted.
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
