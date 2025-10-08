package database

import (
	"gorm.io/gorm"

	"github.com/charlesng/shellcn/internal/models"
)

// AutoMigrate creates or updates the database schema for all models.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.Team{},
		&models.Role{},
		&models.Permission{},
		&models.Session{},
		&models.AuditLog{},
		&models.MFASecret{},
		&models.PasswordResetToken{},
		&models.AuthProvider{},
	)
}

// SeedData populates default roles and authentication providers.
func SeedData(db *gorm.DB) error {
	roles := []models.Role{
		{
			ID:          "admin",
			Name:        "Administrator",
			Description: "Full system access",
			IsSystem:    true,
		},
		{
			ID:          "user",
			Name:        "User",
			Description: "Standard user access",
			IsSystem:    true,
		},
	}

	for _, role := range roles {
		if err := db.Where(models.Role{ID: role.ID}).Attrs(role).FirstOrCreate(&models.Role{}).Error; err != nil {
			return err
		}
	}

	localProvider := models.AuthProvider{
		Type:              "local",
		Name:              "Local Authentication",
		Enabled:           true,
		AllowRegistration: false,
		Description:       "Username and password authentication",
		Icon:              "key",
	}
	if err := db.Where(models.AuthProvider{Type: localProvider.Type}).Attrs(localProvider).FirstOrCreate(&models.AuthProvider{}).Error; err != nil {
		return err
	}

	inviteProvider := models.AuthProvider{
		Type:                     "invite",
		Name:                     "Email Invitation",
		Enabled:                  false,
		RequireEmailVerification: true,
		Description:              "Invite users via email",
		Icon:                     "mail",
	}
	if err := db.Where(models.AuthProvider{Type: inviteProvider.Type}).Attrs(inviteProvider).FirstOrCreate(&models.AuthProvider{}).Error; err != nil {
		return err
	}

	return nil
}
