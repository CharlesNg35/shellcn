package models

import "time"

// ProtocolAvailability gates whether a protocol plugin can be used, and by whom.
type ProtocolAvailability string

const (
	// ProtocolEnabled exposes the protocol to every user (the default).
	ProtocolEnabled ProtocolAvailability = "enabled"
	// ProtocolAdminOnly hides the protocol from non-admins and blocks them from
	// connecting with it.
	ProtocolAdminOnly ProtocolAvailability = "admin_only"
	// ProtocolDisabled hides the protocol from everyone and blocks connecting.
	ProtocolDisabled ProtocolAvailability = "disabled"
)

// Valid reports whether a is a known availability state.
func (a ProtocolAvailability) Valid() bool {
	switch a {
	case ProtocolEnabled, ProtocolAdminOnly, ProtocolDisabled:
		return true
	default:
		return false
	}
}

// Allows reports whether a user (admin or not) may see and use the protocol. An
// empty/unknown state is treated as enabled so a protocol with no stored setting
// stays available.
func (a ProtocolAvailability) Allows(isAdmin bool) bool {
	switch a {
	case ProtocolDisabled:
		return false
	case ProtocolAdminOnly:
		return isAdmin
	default:
		return true
	}
}

// ProtocolSetting is the admin-managed availability state for one protocol,
// keyed by the plugin name. A protocol with no row defaults to enabled.
type ProtocolSetting struct {
	Protocol     string `gorm:"primaryKey"`
	Availability ProtocolAvailability
	UpdatedAt    time.Time
}

func (ProtocolSetting) TableName() string { return "protocol_settings" }
