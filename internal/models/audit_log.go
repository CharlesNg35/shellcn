package models

import "gorm.io/datatypes"

type AuditLog struct {
	BaseModel

	UserID    *string        `gorm:"type:uuid;index" json:"user_id"`
	User      *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Username  string         `json:"username"`
	Action    string         `gorm:"not null;index" json:"action"`
	Resource  string         `gorm:"index" json:"resource"`
	Result    string         `gorm:"not null" json:"result"`
	IPAddress string         `json:"ip_address"`
	UserAgent string         `json:"user_agent"`
	Metadata  datatypes.JSON `json:"metadata"`
}
