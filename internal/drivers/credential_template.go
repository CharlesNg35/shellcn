package drivers

import "time"

// CredentialTemplater is implemented by drivers that publish a credential template for UI and validation.
type CredentialTemplater interface {
	CredentialTemplate() (*CredentialTemplate, error)
}

// CredentialTemplate describes the fields and metadata required to collect credentials for a driver.
type CredentialTemplate struct {
	DriverID            string
	Version             string
	DisplayName         string
	Description         string
	Fields              []CredentialField
	CompatibleProtocols []string
	DeprecatedAfter     *time.Time
	Metadata            map[string]any
}

// Credential field type constants.
const (
	CredentialFieldTypeString  = "string"
	CredentialFieldTypeSecret  = "secret"
	CredentialFieldTypeFile    = "file"
	CredentialFieldTypeEnum    = "enum"
	CredentialFieldTypeBoolean = "boolean"
	CredentialFieldTypeNumber  = "number"
)

// Credential field input mode constants.
const (
	CredentialInputModeText     = "text"
	CredentialInputModeFile     = "file"
	CredentialInputModeSelect   = "select"
	CredentialInputModePassword = "password"
	CredentialInputModeTextarea = "textarea"
)

// Well-known credential field keys that can be shared across drivers.
const (
	CredentialFieldKeyUsername          = "username"
	CredentialFieldKeyPassword          = "password"
	CredentialFieldKeyAuthMethod        = "auth_method"
	CredentialFieldKeyPrivateKey        = "private_key"
	CredentialFieldKeyPassphrase        = "passphrase"
	CredentialFieldKeyDomain            = "domain"
	CredentialFieldKeyKubeconfig        = "kubeconfig"
	CredentialFieldKeyClientCertificate = "client_certificate"
	CredentialFieldKeyClientKey         = "client_key"
	CredentialFieldKeyToken             = "token"
	CredentialFieldKeyRegistry          = "registry"
	CredentialFieldKeyAccessKey         = "access_key"
	CredentialFieldKeySecretKey         = "secret_key"
	CredentialFieldKeyAPIToken          = "api_token"
	CredentialFieldKeyCACertificate     = "ca_certificate"
	CredentialFieldKeyOtpSecret         = "otp_secret"
)

// CredentialField describes an individual field in a credential template.
type CredentialField struct {
	Key         string
	Label       string
	Type        string
	Required    bool
	Description string
	InputModes  []string
	Placeholder string
	Options     []CredentialOption
	Metadata    map[string]any
	Validation  map[string]any
}

// CredentialOption represents a selectable option for enum-like credential fields.
type CredentialOption struct {
	Value string
	Label string
}
