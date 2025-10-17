package drivers

// ConnectionTemplater is implemented by drivers that expose dynamic connection configuration schemas.
type ConnectionTemplater interface {
	ConnectionTemplate() (*ConnectionTemplate, error)
}

// ConnectionTemplate describes the driver-provided schema for rendering and validating connection inputs.
type ConnectionTemplate struct {
	DriverID    string              `json:"driver_id"`
	Version     string              `json:"version"`
	DisplayName string              `json:"display_name"`
	Description string              `json:"description"`
	Sections    []ConnectionSection `json:"sections"`
	Metadata    map[string]any      `json:"metadata,omitempty"`
}

// ConnectionSection groups related fields within a template.
type ConnectionSection struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Description string            `json:"description,omitempty"`
	Fields      []ConnectionField `json:"fields"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// Connection field type constants.
const (
	ConnectionFieldTypeString     = "string"
	ConnectionFieldTypeMultiline  = "multiline"
	ConnectionFieldTypeNumber     = "number"
	ConnectionFieldTypeBoolean    = "boolean"
	ConnectionFieldTypeSelect     = "select"
	ConnectionFieldTypeTargetHost = "target_host"
	ConnectionFieldTypeTargetPort = "target_port"
	ConnectionFieldTypeJSON       = "json"
)

// ConnectionBindingTarget enumerates supported storage destinations for field values.
const (
	BindingTargetSettings         = "settings"
	BindingTargetMetadata         = "metadata"
	BindingTargetConnectionTarget = "target"
)

// ConnectionField defines an individual field within the template.
type ConnectionField struct {
	Key          string             `json:"key"`
	Label        string             `json:"label"`
	Type         string             `json:"type"`
	Required     bool               `json:"required"`
	Default      any                `json:"default,omitempty"`
	Placeholder  string             `json:"placeholder,omitempty"`
	HelpText     string             `json:"help_text,omitempty"`
	Options      []ConnectionOption `json:"options,omitempty"`
	Binding      *ConnectionBinding `json:"binding,omitempty"`
	Validation   map[string]any     `json:"validation,omitempty"`
	Dependencies []FieldDependency  `json:"dependencies,omitempty"`
	Metadata     map[string]any     `json:"metadata,omitempty"`
}

// ConnectionOption represents selectable values for enum/select style fields.
type ConnectionOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ConnectionBinding indicates where a field value should be persisted.
type ConnectionBinding struct {
	Target   string `json:"target"`
	Path     string `json:"path,omitempty"`
	Index    int    `json:"index,omitempty"`
	Property string `json:"property,omitempty"`
}

// FieldDependency models conditional UI or validation requirements between fields.
type FieldDependency struct {
	Field  string `json:"field"`
	Equals any    `json:"equals"`
}
