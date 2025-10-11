package models

type Role struct {
	BaseModel

	Name        string  `gorm:"uniqueIndex;not null" json:"name"`
	Description string  `json:"description"`
	IsSystem    bool    `gorm:"default:false" json:"is_system"`
	IsTemplate  bool    `gorm:"default:false;index" json:"is_template"`
	TemplateID  *string `gorm:"type:uuid;index" json:"template_id"`

	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	Users       []User       `gorm:"many2many:user_roles;" json:"users,omitempty"`
}
