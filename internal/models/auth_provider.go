package models

type AuthProvider struct {
	BaseModel

	Type    string `gorm:"not null;uniqueIndex" json:"type"`
	Name    string `gorm:"not null" json:"name"`
	Enabled bool   `gorm:"default:false" json:"enabled"`
	Config  string `gorm:"type:json" json:"config"`

	AllowRegistration        bool `gorm:"default:false" json:"allow_registration"`
	RequireEmailVerification bool `gorm:"default:true" json:"require_email_verification"`

	Description string `json:"description"`
	Icon        string `json:"icon"`

	CreatedBy string `gorm:"type:uuid" json:"created_by"`
}

type OIDCConfig struct {
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

type OAuth2Config struct {
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	UserInfoURL  string   `json:"user_info_url"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

type SAMLConfig struct {
	MetadataURL      string            `json:"metadata_url"`
	EntityID         string            `json:"entity_id"`
	SSOURL           string            `json:"sso_url"`
	Certificate      string            `json:"certificate"`
	PrivateKey       string            `json:"private_key"`
	AttributeMapping map[string]string `json:"attribute_mapping"`
}

type LDAPConfig struct {
	Host             string            `json:"host"`
	Port             int               `json:"port"`
	BaseDN           string            `json:"base_dn"`
	BindDN           string            `json:"bind_dn"`
	BindPassword     string            `json:"bind_password"`
	UserFilter       string            `json:"user_filter"`
	UseTLS           bool              `json:"use_tls"`
	SkipVerify       bool              `json:"skip_verify"`
	AttributeMapping map[string]string `json:"attribute_mapping"`
}
