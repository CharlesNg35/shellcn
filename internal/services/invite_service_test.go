package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
	"github.com/charlesng35/shellcn/pkg/mail"
)

func TestInviteServiceGenerateAndRedeem(t *testing.T) {
	db := openInviteTestDB(t)
	current := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	svc, err := NewInviteService(db, nil,
		WithInviteClock(func() time.Time { return current }),
		WithInviteExpiry(24*time.Hour),
	)
	require.NoError(t, err)

	invite, token, link, err := svc.GenerateInvite(context.Background(), "user@example.com", "admin", "")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, link)

	require.Equal(t, "user@example.com", invite.Email)
	require.Nil(t, invite.AcceptedAt)

	accepted, err := svc.RedeemInvite(context.Background(), token)
	require.NoError(t, err)
	require.NotNil(t, accepted.AcceptedAt)

	// Redeeming again should fail with already used.
	_, err = svc.RedeemInvite(context.Background(), token)
	require.ErrorIs(t, err, ErrInviteAlreadyUsed)
}

func TestInviteServiceGenerateWithTeam(t *testing.T) {
	db := openInviteTestDB(t)

	team := &models.Team{Name: "Operations"}
	require.NoError(t, db.Create(team).Error)

	svc, err := NewInviteService(db, nil)
	require.NoError(t, err)

	invite, token, link, err := svc.GenerateInvite(context.Background(), "teamuser@example.com", "admin", team.ID)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, link)
	require.NotNil(t, invite.TeamID)
	require.Equal(t, team.ID, *invite.TeamID)
	require.NotNil(t, invite.Team)
	require.Equal(t, team.Name, invite.Team.Name)
}

func TestInviteServiceTeamInviteAllowsExistingUser(t *testing.T) {
	db := openInviteTestDB(t)

	team := &models.Team{Name: "Platform"}
	require.NoError(t, db.Create(team).Error)

	hashed, err := crypto.HashPassword("ExistingPass123!")
	require.NoError(t, err)

	user := &models.User{
		Username: "existing",
		Email:    "existing@example.com",
		Password: hashed,
	}
	require.NoError(t, db.Create(user).Error)

	svc, err := NewInviteService(db, nil)
	require.NoError(t, err)

	invite, token, link, err := svc.GenerateInvite(context.Background(), user.Email, "admin", team.ID)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, link)
	require.NotNil(t, invite.TeamID)
	require.Equal(t, team.ID, *invite.TeamID)
}

func TestInviteServiceTeamInviteRejectsExistingMember(t *testing.T) {
	db := openInviteTestDB(t)

	team := &models.Team{Name: "Security"}
	require.NoError(t, db.Create(team).Error)

	hashed, err := crypto.HashPassword("MemberPass123!")
	require.NoError(t, err)

	user := &models.User{
		Username: "member",
		Email:    "member@example.com",
		Password: hashed,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Model(team).Association("Users").Append(user))

	svc, err := NewInviteService(db, nil)
	require.NoError(t, err)

	_, _, _, err = svc.GenerateInvite(context.Background(), user.Email, "admin", team.ID)
	require.ErrorIs(t, err, ErrInviteUserAlreadyInTeam)
}

func TestInviteServiceRejectsExistingUserEmail(t *testing.T) {
	db := openInviteTestDB(t)

	user := &models.User{
		Username: "existing-user",
		Email:    "existing@example.com",
		Password: "hashed",
	}
	require.NoError(t, db.Create(user).Error)

	svc, err := NewInviteService(db, nil)
	require.NoError(t, err)

	_, _, _, err = svc.GenerateInvite(context.Background(), "Existing@example.com", "admin", "")
	require.ErrorIs(t, err, ErrInviteEmailInUse)
}

func TestInviteServiceResendAndIssueLink(t *testing.T) {
	db := openInviteTestDB(t)
	current := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	svc, err := NewInviteService(db, nil,
		WithInviteClock(func() time.Time { return current }),
		WithInviteExpiry(6*time.Hour),
	)
	require.NoError(t, err)

	invite, _, _, err := svc.GenerateInvite(context.Background(), "resend@example.com", "admin", "")
	require.NoError(t, err)

	resendInvite, resendToken, resendLink, err := svc.ResendInvite(context.Background(), invite.ID)
	require.NoError(t, err)
	require.NotEmpty(t, resendToken)
	require.NotEmpty(t, resendLink)
	require.Equal(t, invite.ID, resendInvite.ID)
	require.True(t, resendInvite.ExpiresAt.After(current))

	issueInvite, issueToken, issueLink, err := svc.IssueInviteLink(context.Background(), invite.ID)
	require.NoError(t, err)
	require.NotEmpty(t, issueToken)
	require.NotEmpty(t, issueLink)
	require.Equal(t, invite.ID, issueInvite.ID)
	require.NotEqual(t, resendToken, issueToken)
}

func TestInviteServiceExpiry(t *testing.T) {
	db := openInviteTestDB(t)
	current := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	svc, err := NewInviteService(db, nil,
		WithInviteClock(func() time.Time { return current }),
		WithInviteExpiry(time.Hour),
	)
	require.NoError(t, err)

	_, token, _, err := svc.GenerateInvite(context.Background(), "late@example.com", "admin", "")
	require.NoError(t, err)

	current = current.Add(2 * time.Hour)

	_, err = svc.RedeemInvite(context.Background(), token)
	require.ErrorIs(t, err, ErrInviteExpired)
}

func TestInviteServiceSMTPDisabled(t *testing.T) {
	db := openInviteTestDB(t)
	svc, err := NewInviteService(db, &disabledMailer{})
	require.NoError(t, err)

	_, token, link, err := svc.GenerateInvite(context.Background(), "disabled@example.com", "admin", "")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, link)
}

func TestInviteServiceDuplicatePrevention(t *testing.T) {
	db := openInviteTestDB(t)
	current := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	svc, err := NewInviteService(db, nil,
		WithInviteClock(func() time.Time { return current }),
		WithInviteExpiry(24*time.Hour),
	)
	require.NoError(t, err)

	_, _, _, err = svc.GenerateInvite(context.Background(), "dup@example.com", "admin", "")
	require.NoError(t, err)

	_, _, _, err = svc.GenerateInvite(context.Background(), "dup@example.com", "admin", "")
	require.ErrorIs(t, err, ErrInviteAlreadyPending)

	// Advance past expiry; should allow invite again.
	current = current.Add(48 * time.Hour)
	_, _, _, err = svc.GenerateInvite(context.Background(), "dup@example.com", "admin", "")
	require.NoError(t, err)
}

func TestInviteServiceListAndDelete(t *testing.T) {
	db := openInviteTestDB(t)
	current := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	svc, err := NewInviteService(db, nil,
		WithInviteClock(func() time.Time { return current }),
		WithInviteExpiry(24*time.Hour),
	)
	require.NoError(t, err)

	inv1, token1, _, err := svc.GenerateInvite(context.Background(), "pending@example.com", "admin", "")
	require.NoError(t, err)

	// Redeem first invite immediately (marking as accepted)
	_, err = svc.RedeemInvite(context.Background(), token1)
	require.NoError(t, err)

	current = current.Add(48 * time.Hour)
	inv2, _, _, err := svc.GenerateInvite(context.Background(), "new@example.com", "admin", "")
	require.NoError(t, err)

	// pending list should only contain second invite
	list, err := svc.List(context.Background(), "pending", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, inv2.ID, list[0].ID)

	list, err = svc.List(context.Background(), "accepted", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, inv1.ID, list[0].ID)

	list, err = svc.List(context.Background(), "expired", "")
	require.NoError(t, err)
	require.Len(t, list, 0)

	// Delete pending invite
	require.NoError(t, svc.Delete(context.Background(), inv2.ID))

	// Ensure delete prevents removal of accepted invite
	require.ErrorIs(t, svc.Delete(context.Background(), inv1.ID), ErrInviteAlreadyUsed)
}

func openInviteTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.Team{}, &models.User{}, &models.UserInvite{}))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

type disabledMailer struct{}

func (disabledMailer) Send(ctx context.Context, msg mail.Message) error {
	return mail.ErrSMTPDisabled
}
