package plugin

import "encoding/json"

// panelConfigDecoders rebuilds the concrete PanelConfig for a panel type. The
// type is the discriminator, so the wire form needs no extra tag.
var panelConfigDecoders = map[PanelType]func(json.RawMessage) (PanelConfig, error){
	PanelTable:         func(r json.RawMessage) (PanelConfig, error) { return decode[TableConfig](r) },
	PanelFileBrowser:   func(r json.RawMessage) (PanelConfig, error) { return decode[FileBrowserConfig](r) },
	PanelForm:          func(r json.RawMessage) (PanelConfig, error) { return decode[FormPanelConfig](r) },
	PanelDashboard:     func(r json.RawMessage) (PanelConfig, error) { return decode[DashboardConfig](r) },
	PanelMetrics:       func(r json.RawMessage) (PanelConfig, error) { return decode[MetricsConfig](r) },
	PanelGraph:         func(r json.RawMessage) (PanelConfig, error) { return decode[GraphConfig](r) },
	PanelTrace:         func(r json.RawMessage) (PanelConfig, error) { return decode[TraceConfig](r) },
	PanelKV:            func(r json.RawMessage) (PanelConfig, error) { return decode[KVConfig](r) },
	PanelTerminal:      func(r json.RawMessage) (PanelConfig, error) { return decode[TerminalConfig](r) },
	PanelTerminalGrid:  func(r json.RawMessage) (PanelConfig, error) { return decode[TerminalGridConfig](r) },
	PanelCodeEditor:    func(r json.RawMessage) (PanelConfig, error) { return decode[CodeEditorConfig](r) },
	PanelDiff:          func(r json.RawMessage) (PanelConfig, error) { return decode[DiffConfig](r) },
	PanelQueryEditor:   func(r json.RawMessage) (PanelConfig, error) { return decode[QueryEditorConfig](r) },
	PanelHTTPClient:    func(r json.RawMessage) (PanelConfig, error) { return decode[HTTPClientConfig](r) },
	PanelRemoteDesktop: func(r json.RawMessage) (PanelConfig, error) { return decode[RemoteDesktopConfig](r) },
	PanelObjectDetail:  func(r json.RawMessage) (PanelConfig, error) { return decode[ObjectDetailConfig](r) },
	PanelTimeline:      func(r json.RawMessage) (PanelConfig, error) { return decode[TimelineConfig](r) },
	PanelTaskProgress:  func(r json.RawMessage) (PanelConfig, error) { return decode[TaskProgressConfig](r) },
	PanelSplit:         func(r json.RawMessage) (PanelConfig, error) { return decode[SplitConfig](r) },
}

func decode[T PanelConfig](raw json.RawMessage) (PanelConfig, error) {
	var c T
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func decodePanelConfig(t PanelType, raw json.RawMessage) (PanelConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if dec, ok := panelConfigDecoders[t]; ok {
		return dec(raw)
	}
	return nil, nil
}

func (p *Panel) UnmarshalJSON(data []byte) error {
	type wire Panel
	aux := struct {
		*wire
		Config json.RawMessage `json:"config,omitempty"`
	}{wire: (*wire)(p)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	cfg, err := decodePanelConfig(p.Type, aux.Config)
	if err != nil {
		return err
	}
	p.Config = cfg
	return nil
}

func (a *Action) UnmarshalJSON(data []byte) error {
	type wire Action
	aux := struct {
		*wire
		Config json.RawMessage `json:"config,omitempty"`
	}{wire: (*wire)(a)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	cfg, err := decodePanelConfig(a.Panel, aux.Config)
	if err != nil {
		return err
	}
	a.Config = cfg
	return nil
}

func (a *ProjectedAction) UnmarshalJSON(data []byte) error {
	type wire ProjectedAction
	aux := struct {
		*wire
		Config json.RawMessage `json:"config,omitempty"`
	}{wire: (*wire)(a)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	cfg, err := decodePanelConfig(a.Panel, aux.Config)
	if err != nil {
		return err
	}
	a.Config = cfg
	return nil
}
