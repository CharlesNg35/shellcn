package models

import "gorm.io/datatypes"

type AuthProvider struct {
	BaseModel

	Type    string         `gorm:"not null;uniqueIndex" json:"type"`
	Name    string         `gorm:"not null" json:"name"`
	Enabled bool           `gorm:"default:false" json:"enabled"`
	Config  datatypes.JSON `json:"config"`

	AllowRegistration        bool `gorm:"default:false" json:"allow_registration"`
	RequireEmailVerification bool `gorm:"default:true" json:"require_email_verification"`
	AllowPasswordReset       bool `gorm:"default:true" json:"allow_password_reset"`

	Description string `json:"description"`
	Icon        string `json:"icon"`

	CreatedBy string `gorm:"type:uuid" json:"created_by"`
}

// remaining types same

type OIDCConfig struct {
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

type SAMLConfig struct {
	MetadataURL      string            `json:"metadata_url"`
	EntityID         string            `json:"entity_id"`
	SSOURL           string            `json:"sso_url"`
	ACSURL           string            `json:"acs_url"`
	Certificate      string            `json:"certificate"`
	PrivateKey       string            `json:"private_key"`
	AttributeMapping map[string]string `json:"attribute_mapping"`
}

type LDAPConfig struct {
	Host                 string            `json:"host"`
	Port                 int               `json:"port"`
	BaseDN               string            `json:"base_dn"`
	UserBaseDN           string            `json:"user_base_dn"`
	BindDN               string            `json:"bind_dn"`
	BindPassword         string            `json:"bind_password"`
	UserFilter           string            `json:"user_filter"`
	UseTLS               bool              `json:"use_tls"`
	SkipVerify           bool              `json:"skip_verify"`
	AttributeMapping     map[string]string `json:"attribute_mapping"`
	SyncGroups           bool              `json:"sync_groups"`
	GroupBaseDN          string            `json:"group_base_dn"`
	GroupNameAttribute   string            `json:"group_name_attribute"`
	GroupMemberAttribute string            `json:"group_member_attribute"`
	GroupFilter          string            `json:"group_filter"`
}
