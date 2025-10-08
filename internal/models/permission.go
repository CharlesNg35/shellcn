package models

type Permission struct {
	BaseModel

	Module      string `gorm:"not null;index" json:"module"`
	Description string `json:"description"`
	DependsOn   string `gorm:"type:json" json:"depends_on"`
	Implies     string `gorm:"type:json" json:"implies"`

	Roles []Role `gorm:"many2many:role_permissions;" json:"roles,omitempty"`
}
