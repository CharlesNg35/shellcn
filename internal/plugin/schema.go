package plugin

// FieldType enumerates the input widgets a config field can render as.
type FieldType string

const (
	FieldText          FieldType = "text"
	FieldEmail         FieldType = "email"
	FieldURL           FieldType = "url"
	FieldTel           FieldType = "tel"
	FieldNumber        FieldType = "number"
	FieldStepper       FieldType = "stepper"
	FieldSlider        FieldType = "slider"
	FieldPassword      FieldType = "password"
	FieldSelect        FieldType = "select"
	FieldRadio         FieldType = "radio"
	FieldMultiSelect   FieldType = "multiselect"
	FieldFile          FieldType = "file"
	FieldToggle        FieldType = "toggle"
	FieldTextarea      FieldType = "textarea"
	FieldJSON          FieldType = "json"
	FieldDuration      FieldType = "duration"
	FieldCredentialRef FieldType = "credential_ref"
)

const (
	SchemaContextTransport = "$transport"
	SchemaContextProtocol  = "$protocol"
)

// CredentialKind tags the type of secret material a reusable credential holds.
type CredentialKind string

const (
	CredentialTLSClientCert  CredentialKind = "tls_client_cert"
	CredentialDBPassword     CredentialKind = "db_password"
	CredentialAPIToken       CredentialKind = "api_token"
	CredentialCloudAccessKey CredentialKind = "cloud_access_key"
	CredentialBasicAuth      CredentialKind = "basic_auth"
	CredentialBearerToken    CredentialKind = "bearer_token"
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
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Secret      bool      `json:"secret,omitempty"`
	Default     any       `json:"default,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Help        string    `json:"help,omitempty"`
	Options     []Option  `json:"options,omitempty"`
	// OptionsSource populates a select/multiselect/radio field's choices from a
	// route at form-open time (rows -> {value,label}), so a field can offer the
	// live values of a related resource (e.g. a table's real columns) instead of
	// a static list or free-typed name. Its params interpolate ${resource.*} from
	// the form's resource context. Static Options still apply when set.
	OptionsSource *DataSource         `json:"optionsSource,omitempty"`
	Credential    *CredentialSelector `json:"credential,omitempty"`
	VisibleWhen   *Condition          `json:"visibleWhen,omitempty"`
	Validators    []Validator         `json:"validators,omitempty"`
	// Step is the increment for number/slider inputs (defaults to 1). Min/max
	// bounds come from the min/max validators.
	Step any `json:"step,omitempty"`
}

type Group struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}

// Schema is the connection config form: ordered groups of typed fields.
type Schema struct {
	Groups []Group `json:"groups"`
}

// Defaults returns the manifest-declared default values keyed by field.
func (s Schema) Defaults() map[string]any {
	out := map[string]any{}
	for _, group := range s.Groups {
		for _, field := range group.Fields {
			if field.Default != nil {
				out[field.Key] = field.Default
			}
		}
	}
	return out
}

// ValuesWithDefaults returns a copy of values with missing schema defaults
// filled in. Explicit false/zero/empty values are preserved.
func (s Schema) ValuesWithDefaults(values map[string]any) map[string]any {
	out := s.Defaults()
	for key, value := range values {
		out[key] = value
	}
	return out
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
