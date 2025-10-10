package models

type Team struct {
	BaseModel

	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`

	Users []User `gorm:"many2many:user_teams;" json:"users,omitempty"`
	Roles []Role `gorm:"many2many:team_roles;" json:"roles,omitempty"`
}
