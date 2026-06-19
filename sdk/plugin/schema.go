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
	// FieldObject nests Fields; FieldArray repeats Item.
	FieldObject FieldType = "object"
	FieldArray  FieldType = "array"
	// FieldAutocomplete is free text with Options/OptionsSource suggestions.
	FieldAutocomplete FieldType = "autocomplete"
	// FieldMap is repeatable key/value rows whose value type is Item.
	FieldMap FieldType = "map"
)

const (
	SchemaContextTransport = "$transport"
	SchemaContextProtocol  = "$protocol"
)

// CredentialKind tags the type of secret material a reusable credential holds.
type CredentialKind string

const (
	CredentialKindSSHPrivateKey  CredentialKind = "ssh_private_key"
	CredentialKindSSHPassword    CredentialKind = "ssh_password"
	CredentialKindTLSClientCert  CredentialKind = "tls_client_cert"
	CredentialKindDBPassword     CredentialKind = "db_password"
	CredentialKindAPIToken       CredentialKind = "api_token"
	CredentialKindCloudAccessKey CredentialKind = "cloud_access_key"
	CredentialKindBasicAuth      CredentialKind = "basic_auth"
	CredentialKindBearerToken    CredentialKind = "bearer_token"
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
	OpGt       Operator = "gt"
	OpLt       Operator = "lt"
	OpGte      Operator = "gte"
	OpLte      Operator = "lte"
	OpContains Operator = "contains"
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
// field accepts. Use separate credential_ref fields for alternative credential
// types. The field stores only the chosen credential id, never a value.
type CredentialSelector struct {
	Kind      CredentialKind `json:"kind"`
	Protocols []string       `json:"protocols,omitempty"`
}

type Rule struct {
	Field string   `json:"field"`
	Op    Operator `json:"op"`
	Value any      `json:"value,omitempty"`
}

// Condition is a structured visibility predicate. AllOf/AnyOf hold leaf rules;
// All/Any/Not nest sub-conditions so arbitrary boolean logic composes, e.g.
// "(A and B) or (C and not D)".
type Condition struct {
	AllOf []Rule      `json:"allOf,omitempty"`
	AnyOf []Rule      `json:"anyOf,omitempty"`
	All   []Condition `json:"all,omitempty"`
	Any   []Condition `json:"any,omitempty"`
	Not   *Condition  `json:"not,omitempty"`
}

type Validator struct {
	Type    ValidatorType `json:"type"`
	Value   any           `json:"value,omitempty"`
	Message string        `json:"message,omitempty"`
}

type Field struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required,omitempty"`
	Secret   bool      `json:"secret,omitempty"`
	// Public is mainly for credential-kind fields: safe non-secret values that
	// may be returned in credential lists and selectors.
	Public bool `json:"public,omitempty"`
	// Default is a UI hint. In action forms, string defaults may reference
	// the active resource with ${resource.uid} or ${resource.name}.
	Default     any      `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Options     []Option `json:"options,omitempty"`
	// OptionsSource populates choices from a route at form-open time.
	OptionsSource *DataSource         `json:"optionsSource,omitempty"`
	Credential    *CredentialSelector `json:"credential,omitempty"`
	VisibleWhen   *Condition          `json:"visibleWhen,omitempty"`
	Validators    []Validator         `json:"validators,omitempty"`
	// Step is the increment for number/slider inputs.
	Step any `json:"step,omitempty"`

	// Composite fields: Fields holds object fields; Item describes array items.
	Fields    []Field `json:"fields,omitempty"`
	Item      *Field  `json:"item,omitempty"`
	MinItems  int     `json:"minItems,omitempty"`
	MaxItems  int     `json:"maxItems,omitempty"`
	ItemLabel string  `json:"itemLabel,omitempty"`
	AddLabel  string  `json:"addLabel,omitempty"`
	// KeyLabel/KeyPlaceholder label the key input of a map field.
	KeyLabel       string `json:"keyLabel,omitempty"`
	KeyPlaceholder string `json:"keyPlaceholder,omitempty"`
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
	found := false
	for _, group := range s.Groups {
		walkFields(group.Fields, func(f Field) {
			if f.Type == FieldFile {
				found = true
			}
		})
	}
	return found
}

// walkFields visits every field, recursing into object Fields and array Items.
func walkFields(fields []Field, visit func(Field)) {
	for _, f := range fields {
		visit(f)
		walkFields(f.Fields, visit)
		if f.Item != nil {
			walkFields([]Field{*f.Item}, visit)
		}
	}
}
