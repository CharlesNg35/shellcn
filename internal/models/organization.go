package models

type Organization struct {
	BaseModel

	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	Settings    string `gorm:"type:json" json:"settings"`

	Users []User `gorm:"foreignKey:OrganizationID" json:"users,omitempty"`
	Teams []Team `gorm:"foreignKey:OrganizationID" json:"teams,omitempty"`
}
