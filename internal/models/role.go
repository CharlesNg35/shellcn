package models

type Role struct {
	BaseModel

	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description"`
	IsSystem    bool   `gorm:"default:false" json:"is_system"`

	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	Users       []User       `gorm:"many2many:user_roles;" json:"users,omitempty"`
}
