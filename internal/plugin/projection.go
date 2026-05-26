package plugin

// Summary is the lightweight catalog entry the connection list needs.
type Summary struct {
	Name        string       `json:"name"`
	Title       string       `json:"title"`
	Icon        Icon         `json:"icon"`
	Category    CategoryInfo `json:"category"`
	Description string       `json:"description,omitempty"`
}

// ProjectedAgentProfile is the render-only view of agent connectivity.
type ProjectedAgentProfile struct {
	Modes    []string `json:"modes"`
	RiskNote string   `json:"riskNote,omitempty"`
}

// ProjectedAction is an action with permission/risk/input/method resolved from
// its route — the browser never sees the permission key or audit-event name.
type ProjectedAction struct {
	ID              string            `json:"id"`
	Label           string            `json:"label"`
	Icon            Icon              `json:"icon,omitzero"`
	RouteID         string            `json:"routeId"`
	Method          Method            `json:"method,omitempty"`
	Params          map[string]string `json:"params,omitempty"`
	Risk            RiskLevel         `json:"risk"`
	RequiresConfirm bool              `json:"requiresConfirm"`
	ConfirmText     string            `json:"confirmText,omitempty"`
	Input           *Schema           `json:"input,omitempty"`
	OnSuccess       *ActionSuccess    `json:"onSuccess,omitempty"`
}

// ProjectedRecording tells the browser which recording options a plugin offers
// for a class, without leaking the server-only stream binding (StreamIDs).
type ProjectedRecording struct {
	Class         RecordingClass    `json:"class"`
	Formats       []RecordingFormat `json:"formats"`
	Authoritative bool              `json:"authoritative"`
	InputCapture  bool              `json:"inputCapture"`
}

// Projection is the render-only contract served to the browser. It mirrors
// projection.ts PluginProjection and excludes handler funcs, raw mount paths,
// permission keys, audit-event names, and any server-only route internals.
type Projection struct {
	APIVersion          int                    `json:"apiVersion"`
	Name                string                 `json:"name"`
	Version             string                 `json:"version"`
	Title               string                 `json:"title"`
	Description         string                 `json:"description"`
	Icon                Icon                   `json:"icon"`
	Category            CategoryInfo           `json:"category"`
	Config              Schema                 `json:"config"`
	Capabilities        []Capability           `json:"capabilities"`
	CredentialKinds     []CredentialKindInfo   `json:"credentialKinds,omitempty"`
	SupportedTransports []Transport            `json:"supportedTransports"`
	Agent               *ProjectedAgentProfile `json:"agent,omitempty"`
	Layout              Layout                 `json:"layout"`
	Tabs                []Tab                  `json:"tabs,omitempty"`
	Tree                []TreeGroup            `json:"tree,omitempty"`
	Resources           []ResourceType         `json:"resources,omitempty"`
	Actions             []ProjectedAction      `json:"actions,omitempty"`
	Streams             []Stream               `json:"streams,omitempty"`
	Recording           []ProjectedRecording   `json:"recording,omitempty"`
}

// BuildProjection derives the browser projection from a validated manifest and
// its indexed routes. Actions resolve their risk/input/method from the route.
func BuildProjection(m Manifest, routes map[string]Route) Projection {
	p := Projection{
		APIVersion:          m.APIVersion,
		Name:                m.Name,
		Version:             m.Version,
		Title:               m.Title,
		Description:         m.Description,
		Icon:                m.Icon,
		Category:            pluginCategoryInfo(m.Category),
		Config:              m.Config,
		Capabilities:        nonNil(m.Capabilities),
		CredentialKinds:     nonNil(m.CredentialKinds),
		SupportedTransports: nonNil(m.SupportedTransports),
		Layout:              m.Layout,
		Tabs:                m.Tabs,
		Tree:                m.Tree,
		Resources:           m.Resources,
		Streams:             m.Streams,
	}

	if m.Agent != nil {
		p.Agent = &ProjectedAgentProfile{
			Modes:    []string{string(m.Agent.Proxy.Mode)},
			RiskNote: string(m.Agent.Proxy.Risk),
		}
	}

	if len(m.Recording) > 0 {
		p.Recording = make([]ProjectedRecording, 0, len(m.Recording))
		for _, c := range m.Recording {
			p.Recording = append(p.Recording, ProjectedRecording{
				Class:         c.Class,
				Formats:       c.Formats,
				Authoritative: c.Authoritative,
				InputCapture:  c.InputCapture,
			})
		}
	}

	if len(m.Actions) > 0 {
		p.Actions = make([]ProjectedAction, 0, len(m.Actions))
		for _, a := range m.Actions {
			pa := ProjectedAction{
				ID:              a.ID,
				Label:           a.Label,
				Icon:            a.Icon,
				RouteID:         a.RouteID,
				Params:          a.Params,
				RequiresConfirm: a.Confirm,
				ConfirmText:     a.ConfirmText,
				OnSuccess:       a.OnSuccess,
			}
			if rt, ok := routes[a.RouteID]; ok {
				pa.Method = rt.Method
				pa.Risk = rt.Risk
				pa.Input = rt.Input
			}
			p.Actions = append(p.Actions, pa)
		}
	}
	return p
}

// nonNil returns an empty (non-nil) slice so the JSON encodes [] not null for
// fields the contract marks as required arrays.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
