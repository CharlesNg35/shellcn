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
	Validation  map[string]any
}

// CredentialOption represents a selectable option for enum-like credential fields.
type CredentialOption struct {
	Value string
	Label string
}
