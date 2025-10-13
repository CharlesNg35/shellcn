package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	testutil "github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
)

func TestAuditServiceRun(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	root := &models.User{
		Username: "root",
		Email:    "root@example.com",
		Password: "hashed",
		IsRoot:   true,
	}
	require.NoError(t, db.Create(root).Error)

	jwtSecret := "0123456789abcdef0123456789abcdef"
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{
		Secret:         jwtSecret,
		Issuer:         "test-suite",
		AccessTokenTTL: time.Hour,
	})
	require.NoError(t, err)

	cfg := &app.Config{
		Vault: app.VaultConfig{EncryptionKey: jwtSecret},
		Auth: app.AuthConfig{
			JWT: app.JWTSettings{
				Secret: jwtSecret,
				Issuer: "test-suite",
				TTL:    time.Hour,
			},
			Session: app.SessionSettings{
				RefreshTTL:    720 * time.Hour,
				RefreshLength: 48,
			},
		},
	}

	svc := NewAuditService(db, jwtSvc, cfg)
	fixed := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)
	svc.WithClock(func() time.Time { return fixed })

	result := svc.Run(context.Background())
	require.Equal(t, fixed.UTC(), result.CheckedAt)
	require.Len(t, result.Checks, 4)
	require.GreaterOrEqual(t, result.Summary[string(StatusPass)], 2)
}

func TestAuditServiceDetectsMissingRoot(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{
		Secret:         "0123456789abcdef0123456789abcdef",
		Issuer:         "test-suite",
		AccessTokenTTL: time.Hour,
	})
	require.NoError(t, err)

	svc := NewAuditService(db, jwtSvc, &app.Config{})
	result := svc.Run(context.Background())

	var rootCheck *Check
	for i := range result.Checks {
		if result.Checks[i].ID == "root_user_present" {
			rootCheck = &result.Checks[i]
		}
	}

	require.NotNil(t, rootCheck)
	require.Equal(t, StatusFail, rootCheck.Status)
}
