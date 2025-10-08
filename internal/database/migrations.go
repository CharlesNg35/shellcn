package database

import (
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
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
			BaseModel:   models.BaseModel{ID: "admin"},
			Name:        "Administrator",
			Description: "Full system access",
			IsSystem:    true,
		},
		{
			BaseModel:   models.BaseModel{ID: "user"},
			Name:        "User",
			Description: "Standard user access",
			IsSystem:    true,
		},
	}

	for _, role := range roles {
		if err := db.Where(models.Role{BaseModel: models.BaseModel{ID: role.ID}}).Attrs(role).FirstOrCreate(&models.Role{}).Error; err != nil {
			return err
		}
	}

	localProvider := models.AuthProvider{
		BaseModel:         models.BaseModel{ID: "local"},
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
		BaseModel:                models.BaseModel{ID: "invite"},
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
