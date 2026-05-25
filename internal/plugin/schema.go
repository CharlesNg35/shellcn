package plugin

// FieldType enumerates the input widgets a config field can render as.
type FieldType string

const (
	FieldText          FieldType = "text"
	FieldNumber        FieldType = "number"
	FieldPassword      FieldType = "password"
	FieldSelect        FieldType = "select"
	FieldMultiSelect   FieldType = "multiselect"
	FieldFile          FieldType = "file"
	FieldToggle        FieldType = "toggle"
	FieldTextarea      FieldType = "textarea"
	FieldJSON          FieldType = "json"
	FieldDuration      FieldType = "duration"
	FieldCredentialRef FieldType = "credential_ref"
)

// CredentialKind tags the type of secret material a reusable credential holds.
type CredentialKind string

const (
	CredentialSSHPrivateKey CredentialKind = "ssh_private_key"
	CredentialSSHPassword   CredentialKind = "ssh_password"
	CredentialTLSClientCert CredentialKind = "tls_client_cert"
	CredentialDBPassword    CredentialKind = "db_password"
	CredentialAPIToken      CredentialKind = "api_token"
)

// Operator is the comparison used by a structured visibility Rule.
type Operator string

const (
	OpEq       Operator = "eq"
	OpNeq      Operator = "neq"
	OpIn       Operator = "in"
	OpNin      Operator = "nin"
	OpEmpty    Operator = "empty"
	OpNotEmpty Operator = "notEmpty"
)

// ValidatorType is the kind of server-side check applied to a field value.
type ValidatorType string

const (
	ValidatorMin   ValidatorType = "min"
	ValidatorMax   ValidatorType = "max"
	ValidatorRegex ValidatorType = "regex"
	ValidatorOneOf ValidatorType = "oneOf"
)

type Option struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// CredentialSelector constrains which reusable credentials a credential_ref
// field accepts. The field stores only the chosen credential id, never a value.
type CredentialSelector struct {
	Kinds     []CredentialKind `json:"kinds"`
	Protocols []string         `json:"protocols,omitempty"`
	Required  bool             `json:"required,omitempty"`
}

type Rule struct {
	Field string   `json:"field"`
	Op    Operator `json:"op"`
	Value any      `json:"value,omitempty"`
}

// Condition is a structured visibility predicate — never a string expression.
type Condition struct {
	AllOf []Rule `json:"allOf,omitempty"`
	AnyOf []Rule `json:"anyOf,omitempty"`
}

type Validator struct {
	Type    ValidatorType `json:"type"`
	Value   any           `json:"value,omitempty"`
	Message string        `json:"message,omitempty"`
}

type Field struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Type        FieldType           `json:"type"`
	Required    bool                `json:"required,omitempty"`
	Secret      bool                `json:"secret,omitempty"`
	Default     any                 `json:"default,omitempty"`
	Placeholder string              `json:"placeholder,omitempty"`
	Help        string              `json:"help,omitempty"`
	Options     []Option            `json:"options,omitempty"`
	Credential  *CredentialSelector `json:"credential,omitempty"`
	VisibleWhen *Condition          `json:"visibleWhen,omitempty"`
	Validators  []Validator         `json:"validators,omitempty"`
}

type Group struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}

// Schema is the connection config form: ordered groups of typed fields.
type Schema struct {
	Groups []Group `json:"groups"`
}

// HasFileField reports whether the schema can submit browser File values and
// therefore requires multipart/form-data binding at the server boundary.
func (s Schema) HasFileField() bool {
	for _, group := range s.Groups {
		for _, field := range group.Fields {
			if field.Type == FieldFile {
				return true
			}
		}
	}
	return false
}
