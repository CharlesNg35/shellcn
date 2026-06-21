package plugin

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// CurrentAPIVersion is the plugin contract version this core supports.
const CurrentAPIVersion = 1

var validRisks = map[RiskLevel]bool{
	RiskSafe: true, RiskWrite: true, RiskDestructive: true, RiskPrivileged: true,
}

var validMethods = map[Method]bool{
	MethodGet: true, MethodPost: true, MethodPut: true,
	MethodPatch: true, MethodDelete: true, MethodWS: true,
}

var pluginNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Validate checks a manifest + its routes at registration, returning an
// aggregate of all actionable problems found (not just the first).
func Validate(m Manifest, routes []Route) error {
	return ValidateWithCredentialKinds(m, routes, mustCredentialKindSet(builtInCredentialKindCatalog))
}

// ValidateWithCredentialKinds checks a manifest against an existing credential
// catalog. Plugin-declared kinds are added to a local copy before credential_ref
// selectors are validated, so a plugin may use the kinds it declares.
func ValidateWithCredentialKinds(m Manifest, routes []Route, existing CredentialKindCatalog) error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}
	if existing == nil {
		existing = mustCredentialKindSet(builtInCredentialKindCatalog)
	}
	catalog, err := NewCredentialKindSet(credentialKindDefinitions(existing.CredentialKinds()))
	if err != nil {
		add("credential kind catalog is invalid: %v", err)
		catalog = mustCredentialKindSet(nil)
	}
	declaredCredentialKinds := map[CredentialKind]bool{}
	for _, info := range m.CredentialKinds {
		if err := catalog.add(info); err != nil {
			add("%v", err)
		}
		declaredCredentialKinds[normalizeCredentialKindInfo(info).Kind] = true
	}

	if m.APIVersion != CurrentAPIVersion {
		add("APIVersion %d unsupported (core supports %d)", m.APIVersion, CurrentAPIVersion)
	}
	if m.Name == "" {
		add("Name is required")
	} else if !pluginNamePattern.MatchString(m.Name) {
		add("Name %q is invalid; use lowercase letters, digits, underscores, or hyphens, starting with a letter", m.Name)
	}
	if m.Title == "" {
		add("Title is required")
	}
	if m.Category == "" {
		add("Category is required")
	} else if _, ok := CategoryLookup(m.Category); !ok {
		add("Category %q is not a built-in category", m.Category)
	}
	switch m.Layout {
	case LayoutTabs, LayoutSidebarTree, LayoutDashboard:
	case LayoutSingle:
		if len(m.Tabs) == 0 {
			add("Layout %q requires one panel in Tabs", m.Layout)
		}
	default:
		add("Layout %q is not a valid layout", m.Layout)
	}

	if !m.SupportsTransport(TransportDirect) {
		add("SupportedTransports must include %q", TransportDirect)
	}
	if m.SupportsTransport(TransportAgent) && m.Agent == nil {
		add("AgentProfile is required when transport %q is declared", TransportAgent)
	}
	if !m.SupportsTransport(TransportAgent) && m.Agent != nil {
		add("AgentProfile declared but transport %q is not supported", TransportAgent)
	}

	routesByID := validateRoutes(m.Name, routes, add)
	actionIDs := validateActions(m, routesByID, collectTabKeys(m), add)
	validateHeaderActions(m, actionIDs, add)
	validateScope(m, routesByID, add)
	streamsByID := validateStreams(m, routesByID, add)
	validateLayout(m, routesByID, actionIDs, add)
	validateRecording(m, streamsByID, add)
	usedCredentialKinds := validateCredentialSelectors(m.Config, catalog, add)
	for kind := range declaredCredentialKinds {
		if !usedCredentialKinds[kind] {
			add("credential kind %q is declared but not used by any credential_ref selector", kind)
		}
	}

	validateSchemaShape("config", m.Config, add)
	for _, rt := range routes {
		if rt.Input != nil {
			validateSchemaShape("route "+rt.ID+" input", *rt.Input, add)
		}
	}

	return errors.Join(errs...)
}

func validateCredentialSelectors(schema Schema, catalog CredentialKindCatalog, add func(string, ...any)) map[CredentialKind]bool {
	used := map[CredentialKind]bool{}
	for _, group := range schema.Groups {
		walkFields(group.Fields, func(field Field) {
			if field.Type != FieldCredentialRef {
				return
			}
			if field.Credential == nil {
				add("credential_ref field %q is missing Credential selector", field.Key)
				return
			}
			kind := field.Credential.Kind
			if kind == "" {
				add("credential_ref field %q declares no accepted credential kind", field.Key)
				return
			}
			used[kind] = true
			if _, ok := catalog.CredentialKindLookup(kind); !ok {
				add("credential_ref field %q declares unknown credential kind %q", field.Key, kind)
			}
		})
	}
	return used
}

// validateSchemaShape checks composite (object/array) field wiring.
func validateSchemaShape(ctx string, schema Schema, add func(string, ...any)) {
	var check func(prefix string, fields []Field)
	check = func(prefix string, fields []Field) {
		seen := map[string]bool{}
		for _, f := range fields {
			where := prefix + "." + f.Key
			if seen[f.Key] {
				add("%s: duplicate field key %q", ctx, where)
			}
			seen[f.Key] = true
			switch f.Type {
			case FieldObject:
				if len(f.Fields) == 0 {
					add("%s: object field %q declares no fields", ctx, where)
				}
				check(where, f.Fields)
			case FieldArray:
				if f.Item == nil {
					add("%s: array field %q declares no item", ctx, where)
					continue
				}
				if f.MaxItems != 0 && f.MinItems > f.MaxItems {
					add("%s: array field %q has minItems > maxItems", ctx, where)
				}
				check(where+"[]", []Field{*f.Item})
			case FieldMap:
				if f.Item == nil {
					add("%s: map field %q declares no value item", ctx, where)
					continue
				}
				check(where+"{}", []Field{*f.Item})
			default:
				if len(f.Fields) > 0 || f.Item != nil {
					add("%s: field %q is %q but declares composite sub-fields", ctx, where, f.Type)
				}
			}
		}
	}
	for _, group := range schema.Groups {
		check(group.Name, group.Fields)
	}
}

// validateRoutes checks route shape and returns the set of route ids.
func validateRoutes(pluginName string, routes []Route, add func(string, ...any)) map[string]Route {
	ids := make(map[string]Route, len(routes))
	routePrefix := pluginName + "."
	for _, rt := range routes {
		if rt.ID == "" {
			add("a route is missing an ID")
			continue
		}
		if pluginName != "" && !strings.HasPrefix(rt.ID, routePrefix) {
			add("route %q must be namespaced under plugin %q (expected prefix %q)", rt.ID, pluginName, routePrefix)
		}
		if _, exists := ids[rt.ID]; exists {
			add("duplicate route ID %q", rt.ID)
		}
		ids[rt.ID] = rt

		if !validMethods[rt.Method] {
			add("route %q has invalid method %q", rt.ID, rt.Method)
		}
		if rt.Permission == "" {
			add("route %q is missing a Permission", rt.ID)
		}
		if !validRisks[rt.Risk] {
			add("route %q has invalid risk %q", rt.ID, rt.Risk)
		}
		if rt.Method == MethodWS {
			if rt.Stream == nil {
				add("WS route %q is missing a Stream handler", rt.ID)
			}
		} else if rt.Handle == nil {
			add("route %q is missing a Handle func", rt.ID)
		}
	}
	return ids
}

// validateActions checks action ids are unique and reference existing routes.
func validateActions(m Manifest, routes map[string]Route, tabs map[string]bool, add func(string, ...any)) map[string]bool {
	ids := make(map[string]bool, len(m.Actions))
	for _, a := range m.Actions {
		if a.ID == "" {
			add("an action is missing an ID")
			continue
		}
		if ids[a.ID] {
			add("duplicate action ID %q", a.ID)
		}
		ids[a.ID] = true
		if a.RouteID == "" {
			add("action %q is missing a RouteID", a.ID)
		} else if _, ok := routes[a.RouteID]; !ok {
			add("action %q references unknown route %q", a.ID, a.RouteID)
		}
		if a.OnSuccess != nil && a.OnSuccess.SelectTab != "" && !tabs[a.OnSuccess.SelectTab] {
			add("action %q onSuccess.selectTab references unknown tab %q", a.ID, a.OnSuccess.SelectTab)
		}
		if a.OnSuccess != nil && a.OnSuccess.Navigate != "" && a.OnSuccess.Navigate != NavigateList {
			add("action %q onSuccess.navigate %q is not a known target", a.ID, a.OnSuccess.Navigate)
		}
		if a.OnSuccess != nil {
			for i, effect := range a.OnSuccess.Effects {
				switch effect.Type {
				case ActionEffectTerminalInput:
					input := effect.TerminalInput
					if input == nil {
						add("action %q onSuccess.effects[%d].terminalInput is required", a.ID, i)
						continue
					}
					if input.Tab != "" && !tabs[input.Tab] {
						add("action %q onSuccess.effects[%d].terminalInput.tab references unknown tab %q", a.ID, i, input.Tab)
					}
					if input.Text == "" && input.ResultField == "" {
						add("action %q onSuccess.effects[%d].terminalInput requires text or resultField", a.ID, i)
					}
				case ActionEffectOpenPanel:
					op := effect.OpenPanel
					if op == nil {
						add("action %q onSuccess.effects[%d].openPanel is required", a.ID, i)
						continue
					}
					if op.Open != OpenDock && op.Open != OpenDialog {
						add("action %q onSuccess.effects[%d].openPanel.open must be dock or dialog", a.ID, i)
					}
					if op.Panel == "" {
						add("action %q onSuccess.effects[%d].openPanel declares no panel type", a.ID, i)
					}
				default:
					add("action %q onSuccess.effects[%d].type %q is not known", a.ID, i, effect.Type)
				}
			}
		}
		if (a.Open == OpenDock || a.Open == OpenDialog) && a.Panel == "" {
			add("action %q opens a panel (%s) but declares no panel type", a.ID, a.Open)
		}
	}
	return ids
}

// validateHeaderActions checks the header references existing actions.
func validateHeaderActions(m Manifest, actionIDs map[string]bool, add func(string, ...any)) {
	for _, id := range m.HeaderActions {
		if !actionIDs[id] {
			add("headerAction %q references unknown action", id)
		}
	}
}

// validateScope checks each scope filter is well-formed: unique param, a label,
// a resolvable optionsSource, choices where the control needs them, and the
// separator a multiselect's handler will split on.
func validateScope(m Manifest, routes map[string]Route, add func(string, ...any)) {
	seen := make(map[string]bool, len(m.Scope))
	for _, s := range m.Scope {
		if s.Param == "" {
			add("a scope filter is missing a param")
			continue
		}
		if seen[s.Param] {
			add("duplicate scope filter param %q", s.Param)
		}
		seen[s.Param] = true
		if s.Label == "" {
			add("scope filter %q is missing a label", s.Param)
		}
		if s.OptionsSource != nil {
			if _, ok := routes[s.OptionsSource.RouteID]; !ok {
				add("scope filter %q optionsSource references unknown route %q", s.Param, s.OptionsSource.RouteID)
			}
		}
		if s.WatchSource != nil {
			rt, ok := routes[s.WatchSource.RouteID]
			if !ok {
				add("scope filter %q watchSource references unknown route %q", s.Param, s.WatchSource.RouteID)
			} else if rt.Method != MethodWS {
				add("scope filter %q watchSource route %q must use WS", s.Param, s.WatchSource.RouteID)
			}
		}
		hasChoices := len(s.Options) > 0 || s.OptionsSource != nil
		switch s.Control {
		case "", ScopeSelect:
			if !hasChoices {
				add("scope filter %q has no choices (set options or optionsSource)", s.Param)
			}
		case ScopeSearch:
			if s.Multiple {
				add("scope filter %q is a search but declares multiple", s.Param)
			}
			// free text — needs no choices.
		case ScopeToggle:
			if s.Multiple {
				add("scope filter %q is a toggle but declares multiple", s.Param)
			}
			if s.AllowCustom {
				add("scope filter %q is a toggle but allows custom values", s.Param)
			}
			if len(s.Options) == 0 {
				add("scope filter %q is a toggle but declares no option for its on-value", s.Param)
			}
		case ScopeAutoComplete:
			if !hasChoices && !s.AllowCustom {
				add("scope filter %q has no choices (set options or optionsSource)", s.Param)
			}
		default:
			add("scope filter %q has invalid control %q", s.Param, s.Control)
		}
	}
}

func collectTabKeys(m Manifest) map[string]bool {
	keys := map[string]bool{}
	for _, tab := range m.Tabs {
		if tab.Key != "" {
			keys[tab.Key] = true
		}
	}
	for _, res := range m.Resources {
		for _, tab := range res.Detail.Tabs {
			if tab.Key != "" {
				keys[tab.Key] = true
			}
		}
	}
	return keys
}

// validateStreams checks stream ids reference existing WS routes and returns the
// declared streams indexed by id (for recording validation).
func validateStreams(m Manifest, routes map[string]Route, add func(string, ...any)) map[string]Stream {
	seen := make(map[string]Stream, len(m.Streams))
	for _, s := range m.Streams {
		if s.ID == "" {
			add("a stream is missing an ID")
			continue
		}
		if _, dup := seen[s.ID]; dup {
			add("duplicate stream ID %q", s.ID)
		}
		seen[s.ID] = s
		switch {
		case s.RouteID == "":
			add("stream %q is missing a RouteID", s.ID)
		case routes[s.RouteID].ID == "":
			add("stream %q references unknown route %q", s.ID, s.RouteID)
		case routes[s.RouteID].Method != MethodWS:
			add("stream %q references non-WS route %q", s.ID, s.RouteID)
		}
	}
	return seen
}

// validateRecording checks recording declarations: known classes (no dupes),
// class/format compatibility, and that StreamIDs reference declared streams whose
// kind matches the class. It rejects shapes that could enable unsupported
// recording, but never asserts a default policy (recording stays off by default).
func validateRecording(m Manifest, streams map[string]Stream, add func(string, ...any)) {
	seenClass := map[RecordingClass]bool{}
	for _, c := range m.Recording {
		switch c.Class {
		case RecordingTerminal, RecordingDesktop:
		default:
			add("recording capability has invalid class %q", c.Class)
			continue
		}
		if seenClass[c.Class] {
			add("duplicate recording class %q", c.Class)
		}
		seenClass[c.Class] = true

		if len(c.Formats) == 0 {
			add("recording class %q declares no formats", c.Class)
		}
		if len(c.StreamIDs) == 0 {
			add("recording class %q declares no streams", c.Class)
		}
		for _, f := range c.Formats {
			if !recordingFormatValidForClass(c.Class, f) {
				add("recording class %q does not support format %q", c.Class, f)
			}
		}

		wantKind := streamKindForClass(c.Class)
		for _, id := range c.StreamIDs {
			s, ok := streams[id]
			if !ok {
				add("recording class %q references unknown stream %q", c.Class, id)
				continue
			}
			if s.Kind != wantKind {
				add("recording class %q references stream %q with incompatible kind %q", c.Class, id, s.Kind)
			}
		}
	}
}

func recordingFormatValidForClass(class RecordingClass, f RecordingFormat) bool {
	switch class {
	case RecordingTerminal:
		return terminalFormats[f]
	case RecordingDesktop:
		return desktopFormats[f]
	default:
		return false
	}
}

func streamKindForClass(class RecordingClass) StreamKind {
	if class == RecordingDesktop {
		return StreamDesktop
	}
	return StreamTerminal
}

// validateLayout checks every DataSource RouteID and ActionID reference resolves.
func validateLayout(m Manifest, routes map[string]Route, actionIDs map[string]bool, add func(string, ...any)) {
	checkDataSource := func(ctx string, ds DataSource) (Route, bool) {
		if ds.RouteID == "" {
			add("%s is missing a RouteID", ctx)
			return Route{}, false
		}
		rt, ok := routes[ds.RouteID]
		if !ok {
			add("%s references unknown route %q", ctx, ds.RouteID)
			return Route{}, false
		}
		if ds.Method != "" && ds.Method != rt.Method {
			add("%s declares method %q but route %q uses %q", ctx, ds.Method, ds.RouteID, rt.Method)
		}
		return rt, true
	}
	checkReadSource := func(ctx string, ds DataSource) {
		rt, ok := checkDataSource(ctx, ds)
		if !ok {
			return
		}
		if rt.Method != MethodGet {
			add("%s references route %q with invalid read method %q", ctx, ds.RouteID, rt.Method)
		}
	}
	checkWriteSource := func(ctx string, ds DataSource) {
		rt, ok := checkDataSource(ctx, ds)
		if !ok {
			return
		}
		if rt.Method == MethodGet || rt.Method == MethodWS {
			add("%s references route %q with invalid write method %q", ctx, ds.RouteID, rt.Method)
		}
	}
	checkRouteID := func(ctx string, routeID string) {
		if routeID == "" {
			return
		}
		if _, ok := routes[routeID]; !ok {
			add("%s references unknown route %q", ctx, routeID)
		}
	}
	checkWriteRouteID := func(ctx string, routeID string) {
		if routeID == "" {
			return
		}
		rt, ok := routes[routeID]
		if !ok {
			add("%s references unknown route %q", ctx, routeID)
			return
		}
		if rt.Method == MethodGet || rt.Method == MethodWS {
			add("%s references route %q with invalid write method %q", ctx, routeID, rt.Method)
		}
	}
	checkMultipartRouteID := func(ctx string, routeID string) {
		if routeID == "" {
			return
		}
		rt, ok := routes[routeID]
		if !ok {
			add("%s references unknown route %q", ctx, routeID)
			return
		}
		if rt.Method == MethodGet || rt.Method == MethodWS {
			add("%s references route %q with invalid upload method %q", ctx, routeID, rt.Method)
		}
		if rt.Input == nil || !rt.Input.HasFileField() {
			add("%s references route %q without a file input schema", ctx, routeID)
		}
	}
	checkStreamSource := func(ctx string, ds *DataSource) {
		if ds == nil {
			add("%s is missing a source", ctx)
			return
		}
		rt, ok := checkDataSource(ctx, *ds)
		if !ok {
			return
		}
		if rt.Method != MethodWS {
			add("%s references route %q with invalid stream method %q", ctx, ds.RouteID, rt.Method)
		}
	}
	checkPanelSource := func(ctx string, panel PanelType, ds *DataSource) {
		switch panel {
		case PanelTerminal, PanelTerminalGrid, PanelLogStream, PanelMetrics, PanelQueryEditor, PanelRemoteDesktop, PanelTaskProgress, PanelCanvas:
			checkStreamSource(ctx, ds)
		default:
			if ds != nil {
				checkReadSource(ctx, *ds)
			}
		}
	}
	checkActionIDs := func(ctx string, ids []string) {
		for _, id := range ids {
			if !actionIDs[id] {
				add("%s references unknown action %q", ctx, id)
			}
		}
	}
	checkBridgeRoute := func(ctx string, route WasmBridgeRoute) {
		checkRouteID(ctx, route.RouteID)
		if rt, ok := routes[route.RouteID]; ok {
			if rt.Method == MethodWS {
				add("%s references stream route %q", ctx, route.RouteID)
			}
			if route.Method != "" && route.Method != rt.Method {
				add("%s declares method %q but route %q uses %q", ctx, route.Method, route.RouteID, rt.Method)
			}
		}
		if route.Method == MethodWS {
			add("%s method cannot be WS", ctx)
		}
	}
	// checkConfig/checkPanel collapse the otherwise-repeated 11-argument call (a
	// variant inherits the panel's source). ctx strings match the originals so
	// error messages are unchanged.
	checkConfig := func(ctx string, config PanelConfig) {
		checkPanelConfigRoutes(ctx, config, checkReadSource, checkWriteSource, checkRouteID, checkWriteRouteID, checkMultipartRouteID, checkStreamSource, checkPanelSource, checkActionIDs, checkBridgeRoute, add)
	}
	checkPanel := func(ctx string, p Panel) {
		checkPanelSource(ctx+" source", p.Type, p.Source)
		checkConfig(ctx, p.Config)
		for i, variant := range p.Variants {
			checkPanelSource(fmt.Sprintf("%s variants[%d] source", ctx, i), variant.Type, p.Source)
			checkConfig(fmt.Sprintf("%s variants[%d]", ctx, i), variant.Config)
		}
	}
	checkTabs := func(ctx string, tabs []Panel) {
		for _, t := range tabs {
			checkPanel(fmt.Sprintf("%s tab %q", ctx, t.Key), t)
		}
	}

	checkTabs("connection", m.Tabs)
	for _, action := range m.Actions {
		for i, effect := range action.OnSuccess.effects() {
			if effect.Type != ActionEffectOpenPanel || effect.OpenPanel == nil {
				continue
			}
			ctx := fmt.Sprintf("action %q onSuccess.effects[%d].openPanel", action.ID, i)
			checkPanelSource(ctx+" source", effect.OpenPanel.Panel, effect.OpenPanel.Source)
			checkConfig(ctx, effect.OpenPanel.Config)
		}
		if action.Panel == "" || (action.Open != OpenDock && action.Open != OpenDialog) {
			continue
		}
		checkConfig(fmt.Sprintf("action %q panel", action.ID), action.Config)
	}
	resourceKinds := map[string]bool{}
	for _, rt := range m.Resources {
		resourceKinds[rt.Kind] = true
	}
	for _, g := range m.Tree {
		// A group is either expandable (declares a children Source) or a leaf
		// (opens its ResourceKind list or Ref detail directly).
		switch {
		case g.Source.RouteID != "":
			checkReadSource(fmt.Sprintf("tree group %q source", g.Key), g.Source)
		case g.ResourceKind == "" && g.Ref == nil:
			add("tree group %q must declare a source, resourceKind, or ref", g.Key)
		}
		if g.ResourceKind != "" && !resourceKinds[g.ResourceKind] {
			add("tree group %q references unknown resource kind %q", g.Key, g.ResourceKind)
		}
		if g.Ref != nil && g.Ref.Kind != "" && !resourceKinds[g.Ref.Kind] {
			add("tree group %q ref references unknown resource kind %q", g.Key, g.Ref.Kind)
		}
	}
	for _, rt := range m.Resources {
		checkReadSource(fmt.Sprintf("resource %q list", rt.Kind), rt.List)
		if rt.Watch != nil {
			checkStreamSource(fmt.Sprintf("resource %q watch", rt.Kind), rt.Watch)
		}
		if rt.ColumnsSource != nil {
			checkReadSource(fmt.Sprintf("resource %q columnsSource", rt.Kind), *rt.ColumnsSource)
		}
		checkActionIDs(fmt.Sprintf("resource %q toolbar", rt.Kind), rt.Actions.Toolbar)
		checkActionIDs(fmt.Sprintf("resource %q row", rt.Kind), rt.Actions.Row)
		checkActionIDs(fmt.Sprintf("resource %q detail", rt.Kind), rt.Actions.Detail)
		// Selectable without Row actions = checkboxes that can never trigger an
		// action (dead selection). Row implies selectable, so Selectable is only
		// meaningful with Row — otherwise it is a bug.
		if rt.Actions.Selectable && len(rt.Actions.Row) == 0 {
			add("resource %q is Selectable but declares no Row actions (dead selection)", rt.Kind)
		}
		if rt.Detail.DefaultTab != "" {
			found := false
			for _, tab := range rt.Detail.Tabs {
				if tab.Key == rt.Detail.DefaultTab {
					found = true
					break
				}
			}
			if !found {
				add("resource %q detail defaultTab references unknown tab %q", rt.Kind, rt.Detail.DefaultTab)
			}
		}
		checkTabs(fmt.Sprintf("resource %q detail", rt.Kind), rt.Detail.Tabs)
	}
}

func validateWriteConfigMethod(ctx string, method Method, add func(string, ...any)) {
	if method == "" {
		return
	}
	switch method {
	case MethodPost, MethodPut, MethodPatch, MethodDelete:
	default:
		add("%s has invalid write method %q", ctx, method)
	}
}

func checkSaveFeedback(ctx string, toast *SaveToast, dismiss SaveDismiss, add func(string, ...any)) {
	switch dismiss {
	case SaveDismissNone, SaveDismissClose:
	default:
		add("%s saveDismiss %q is not a known value", ctx, dismiss)
	}
	if toast == nil || toast.Severity == "" {
		return
	}
	switch toast.Severity {
	case SeverityInfo, SeveritySuccess, SeverityWarn, SeverityDanger, SeveritySecondary:
	default:
		add("%s saveToast.severity %q is not a known severity", ctx, toast.Severity)
	}
}

// checkPanelConfigRoutes validates the route/action IDs a typed panel config
// references by switching on the concrete config — no string-key introspection.
func checkPanelConfigRoutes(
	ctx string,
	config PanelConfig,
	checkReadSource func(string, DataSource),
	checkWriteSource func(string, DataSource),
	checkRouteID func(string, string),
	checkWriteRouteID func(string, string),
	checkMultipartRouteID func(string, string),
	checkStreamSource func(string, *DataSource),
	checkPanelSource func(string, PanelType, *DataSource),
	checkActionIDs func(string, []string),
	checkBridgeRoute func(string, WasmBridgeRoute),
	add func(string, ...any),
) {
	// recursePanel checks a nested panel (dashboard cell, split child) and its
	// variants, threading the same checkers through the recursion.
	recurseConfig := func(ctx string, cfg PanelConfig) {
		checkPanelConfigRoutes(ctx, cfg, checkReadSource, checkWriteSource, checkRouteID, checkWriteRouteID, checkMultipartRouteID, checkStreamSource, checkPanelSource, checkActionIDs, checkBridgeRoute, add)
	}
	recursePanel := func(ctx string, p Panel) {
		if p.Type == "" {
			add("%s is missing a panel type", ctx)
		}
		checkPanelSource(ctx+" source", p.Type, p.Source)
		recurseConfig(ctx, p.Config)
		for i, variant := range p.Variants {
			checkPanelSource(fmt.Sprintf("%s variants[%d] source", ctx, i), variant.Type, p.Source)
			recurseConfig(fmt.Sprintf("%s variants[%d]", ctx, i), variant.Config)
		}
	}
	switch c := config.(type) {
	case TableConfig:
		validateTableConfig(ctx, c, add)
		if c.ColumnsSource != nil {
			checkReadSource(ctx+" columnsSource", *c.ColumnsSource)
		}
		if c.Watch != nil {
			checkStreamSource(ctx+" watch", c.Watch)
		}
		if c.Insert != nil {
			checkWriteSource(ctx+" insert", *c.Insert)
		}
		if c.Update != nil {
			checkWriteSource(ctx+" update", *c.Update)
		}
		if c.Delete != nil {
			checkWriteSource(ctx+" delete", *c.Delete)
		}
		checkActionIDs(ctx+" actionIds", c.ActionIDs)
		checkActionIDs(ctx+" rowActionIds", c.RowActionIDs)
	case FileBrowserConfig:
		checkRouteID(ctx+" routes.read", c.Routes.Read)
		checkRouteID(ctx+" routes.download", c.Routes.Download)
		checkWriteRouteID(ctx+" routes.write", c.Routes.Write)
		checkMultipartRouteID(ctx+" upload.routeId", c.Upload.RouteID)
		checkWriteRouteID(ctx+" routes.mkdir", c.Routes.Mkdir)
		checkWriteRouteID(ctx+" routes.rename", c.Routes.Rename)
		checkWriteRouteID(ctx+" routes.delete", c.Routes.Delete)
		checkWriteRouteID(ctx+" routes.move", c.Routes.Move)
		checkWriteRouteID(ctx+" routes.copy", c.Routes.Copy)
		checkWriteRouteID(ctx+" routes.chmod", c.Routes.Chmod)
		checkRouteID(ctx+" routes.archive", c.Routes.Archive)
	case FormPanelConfig:
		checkWriteRouteID(ctx+" submitRouteId", c.SubmitRouteID)
		validateWriteConfigMethod(ctx+" submitMethod", c.SubmitMethod, add)
		checkSaveFeedback(ctx, c.SaveToast, c.SaveDismiss, add)
	case CodeEditorConfig:
		checkWriteRouteID(ctx+" saveRouteId", c.SaveRouteID)
		validateWriteConfigMethod(ctx+" saveMethod", c.SaveMethod, add)
		if c.Watch != nil {
			checkStreamSource(ctx+" watch", c.Watch)
		}
		checkSaveFeedback(ctx, c.SaveToast, c.SaveDismiss, add)
	case LogStreamConfig:
		for i, ctrl := range c.Controls {
			if ctrl.OptionsSource != nil {
				checkReadSource(fmt.Sprintf("%s control[%d] optionsSource", ctx, i), *ctrl.OptionsSource)
			}
		}
	case ObjectDetailConfig:
		if c.Watch != nil {
			checkStreamSource(ctx+" watch", c.Watch)
		}
	case TimelineConfig:
		if c.Watch != nil {
			checkStreamSource(ctx+" watch", c.Watch)
		}
	case QueryEditorConfig:
		checkWriteRouteID(ctx+" cancelRouteId", c.CancelRouteID)
		checkRouteID(ctx+" completionRouteId", c.CompletionRouteID)
	case CanvasConfig:
		switch c.ScaleMode {
		case "", CanvasScaleResize, CanvasScaleFit, CanvasScaleScroll:
		default:
			add("%s scaleMode %q is not supported", ctx, c.ScaleMode)
		}
		if c.Width < 0 || c.Height < 0 {
			add("%s width and height must be non-negative", ctx)
		}
		if (c.Width == 0) != (c.Height == 0) {
			add("%s width and height must be declared together", ctx)
		}
		if (c.ScaleMode == CanvasScaleFit || c.ScaleMode == CanvasScaleScroll) && (c.Width <= 0 || c.Height <= 0) {
			add("%s scaleMode %s requires positive width and height", ctx, c.ScaleMode)
		}
	case KVConfig:
		checkWriteRouteID(ctx+" createRouteId", c.CreateRouteID)
		checkRouteID(ctx+" readRouteId", c.ReadRouteID)
		checkWriteRouteID(ctx+" writeRouteId", c.WriteRouteID)
		checkWriteRouteID(ctx+" deleteRouteId", c.DeleteRouteID)
	case HTTPClientConfig:
		checkWriteRouteID(ctx+" executeRouteId", c.ExecuteRouteID)
	case TaskProgressConfig:
		checkWriteRouteID(ctx+" cancelRouteId", c.CancelRouteID)
		checkWriteRouteID(ctx+" retryRouteId", c.RetryRouteID)
	case WasmConfig:
		assetPaths := map[string]bool{}
		if c.Entry == "" {
			add("%s entry is required", ctx)
		}
		switch c.Runtime {
		case "", WasmRuntimeGo, WasmRuntimeGeneric:
		default:
			add("%s runtime %q is not supported", ctx, c.Runtime)
		}
		switch c.ScaleMode {
		case "", WasmScaleResize, WasmScaleFit, WasmScaleScroll:
		default:
			add("%s scaleMode %q is not supported", ctx, c.ScaleMode)
		}
		if c.Width < 0 || c.Height < 0 {
			add("%s width and height must be non-negative", ctx)
		}
		if (c.Width == 0) != (c.Height == 0) {
			add("%s width and height must be declared together", ctx)
		}
		if c.ScaleMode == WasmScaleFit && (c.Width == 0 || c.Height == 0) {
			add("%s scaleMode fit requires width and height", ctx)
		}
		for i, asset := range c.Assets {
			if asset.Path == "" {
				add("%s assets[%d].path is required", ctx, i)
			} else if assetPaths[asset.Path] {
				add("%s assets[%d].path %q is duplicated", ctx, i, asset.Path)
			}
			assetPaths[asset.Path] = true
			checkReadSource(fmt.Sprintf("%s assets[%d].source", ctx, i), asset.Source)
		}
		if c.Entry != "" && !assetPaths[c.Entry] {
			add("%s entry %q is not declared in assets", ctx, c.Entry)
		}
		for i, script := range c.Boot.Scripts {
			if !assetPaths[script] {
				add("%s boot.scripts[%d] %q is not declared in assets", ctx, i, script)
			}
		}
		for i, route := range c.Bridge.Routes {
			routeCtx := fmt.Sprintf("%s bridge.routes[%d].routeId", ctx, i)
			checkBridgeRoute(routeCtx, route)
		}
		for i, stream := range c.Bridge.Streams {
			checkStreamSource(fmt.Sprintf("%s bridge.streams[%d]", ctx, i), &DataSource{RouteID: stream.RouteID, Method: MethodWS, Params: stream.Params})
		}
	case DashboardConfig:
		for _, cell := range c.Cells {
			recursePanel(fmt.Sprintf("%s cell %q", ctx, cell.Key), cell)
		}
	case SplitConfig:
		for _, child := range c.Panels {
			recursePanel(fmt.Sprintf("%s split panel %q", ctx, child.Key), child.Panel)
		}
	}
}

func validateTableConfig(ctx string, c TableConfig, add func(string, ...any)) {
	if c.Editable && c.Insert == nil && c.Update == nil && c.Delete == nil {
		add("%s editable table must declare insert, update, or delete", ctx)
	}
	for _, key := range c.RowKey {
		if strings.TrimSpace(key) == "" {
			add("%s rowKey contains an empty field", ctx)
		}
	}
	for _, col := range c.Columns {
		validateTableColumn(ctx, col, add)
	}
}

func validateTableColumn(ctx string, col Column, add func(string, ...any)) {
	if strings.TrimSpace(col.Key) == "" {
		add("%s column has an empty key", ctx)
	}
	if col.Editable && col.ReadOnly {
		add("%s column %q cannot be both editable and readOnly", ctx, col.Key)
	}
	if !col.Editable && col.Editor != "" {
		add("%s column %q declares editor without editable", ctx, col.Key)
	}
	switch col.Editor {
	case "":
		if col.Editable {
			add("%s editable column %q must declare editor", ctx, col.Key)
		}
	case ColumnEditorText, ColumnEditorTextarea, ColumnEditorNumber, ColumnEditorToggle, ColumnEditorSelect, ColumnEditorJSON:
	default:
		add("%s column %q has invalid editor %q", ctx, col.Key, col.Editor)
	}
	if col.Editor == ColumnEditorSelect && len(col.Options) == 0 {
		add("%s column %q select editor requires options", ctx, col.Key)
	}
}
