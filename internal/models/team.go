package models

type Team struct {
	BaseModel

	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`

	OrganizationID string        `gorm:"type:uuid;not null" json:"organization_id"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Users          []User        `gorm:"many2many:user_teams;" json:"users,omitempty"`
}
