package database

import (
	"github.com/charlesng35/shellcn/internal/models"
	"gorm.io/gorm"
)

func assignRolePermissions(db *gorm.DB, roleID string, permissionIDs []string) error {
	if len(permissionIDs) == 0 {
		return nil
	}

	var role models.Role
	if err := db.Where("id = ?", roleID).First(&role).Error; err != nil {
		return err
	}

	var perms []models.Permission
	if err := db.Where("id IN ?", permissionIDs).Find(&perms).Error; err != nil {
		return err
	}
	if len(perms) == 0 {
		return nil
	}

	var existing []models.Permission
	if err := db.Model(&role).Association("Permissions").Find(&existing); err != nil {
		return err
	}
	current := make(map[string]struct{}, len(existing))
	for _, perm := range existing {
		current[perm.ID] = struct{}{}
	}

	toAttach := make([]models.Permission, 0, len(perms))
	for _, perm := range perms {
		if _, ok := current[perm.ID]; !ok {
			toAttach = append(toAttach, perm)
		}
	}
	if len(toAttach) == 0 {
		return nil
	}

	return db.Model(&role).Association("Permissions").Append(toAttach)
}
