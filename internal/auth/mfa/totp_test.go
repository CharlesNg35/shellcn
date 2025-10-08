package mfa

import (
	"bytes"
	"encoding/json"
	"image/png"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestGenerateSecretStoresEncryptedData(t *testing.T) {
	db := openTestDB(t)

	user := createTestUser(t, db, "alice")
	key, backup, service := createServiceAndGenerate(t, db, user)

	require.NotNil(t, key)
	require.Len(t, backup, defaultBackupCodeCount)

	var stored models.MFASecret
	require.NoError(t, db.Where("user_id = ?", user.ID).First(&stored).Error)
	require.NotEmpty(t, stored.Secret)
	require.NotEqual(t, key.Secret(), stored.Secret)

	decrypted, err := crypto.Decrypt(stored.Secret, service.encryptionKey)
	require.NoError(t, err)
	require.Equal(t, key.Secret(), string(decrypted))

	var hashed []string
	require.NoError(t, json.Unmarshal([]byte(stored.BackupCodes), &hashed))
	require.Len(t, hashed, defaultBackupCodeCount)
	for i := range hashed {
		require.True(t, crypto.VerifyPassword(hashed[i], backup[i]))
	}
}

func TestVerifyCodeAndUpdateLastUsed(t *testing.T) {
	db := openTestDB(t)
	user := createTestUser(t, db, "bob")
	key, _, service := createServiceAndGenerate(t, db, user)

	code, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	valid, err := service.VerifyCode(user.ID, code)
	require.NoError(t, err)
	require.True(t, valid)

	var stored models.MFASecret
	require.NoError(t, db.Where("user_id = ?", user.ID).First(&stored).Error)
	require.NotNil(t, stored.LastUsedAt)

	valid, err = service.VerifyCode(user.ID, "000000")
	require.NoError(t, err)
	require.False(t, valid)
}

func TestUseBackupCodeConsumesCode(t *testing.T) {
	db := openTestDB(t)
	user := createTestUser(t, db, "carol")
	_, backup, service := createServiceAndGenerate(t, db, user)

	ok, err := service.UseBackupCode(user.ID, backup[0])
	require.NoError(t, err)
	require.True(t, ok)

	count, err := service.RemainingBackupCodes(user.ID)
	require.NoError(t, err)
	require.Equal(t, defaultBackupCodeCount-1, count)

	ok, err = service.UseBackupCode(user.ID, backup[0])
	require.NoError(t, err)
	require.False(t, ok)
}

func TestGenerateQRCode(t *testing.T) {
	db := openTestDB(t)
	user := createTestUser(t, db, "dave")
	key, _, service := createServiceAndGenerate(t, db, user)

	data, err := service.GenerateQRCode(key)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	_, err = png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
}

func createServiceAndGenerate(t *testing.T, db *gorm.DB, user *models.User) (*otp.Key, []string, *TOTPService) {
	t.Helper()

	key := []byte("12345678901234567890123456789012")
	service, err := NewTOTPService(db, key, WithIssuer("ShellCN Test"))
	require.NoError(t, err)

	totpKey, backupCodes, err := service.GenerateSecret(user.ID, user.Username)
	require.NoError(t, err)

	return totpKey, backupCodes, service
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := database.Open(database.Config{Driver: "sqlite"})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(db))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func createTestUser(t *testing.T, db *gorm.DB, username string) *models.User {
	t.Helper()

	hashed, err := crypto.HashPassword("password")
	require.NoError(t, err)

	user := &models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: hashed,
		IsActive: true,
	}

	require.NoError(t, db.Create(user).Error)
	return user
}
