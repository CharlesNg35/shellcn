package plugins

import (
	"encoding/json"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugin/pluginux"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestAllPluginProjectionsMarshal(t *testing.T) {
	for _, p := range allTestPlugins(t) {
		name := p.Manifest().Name
		t.Run(name, func(t *testing.T) {
			proj := plugintest.Projection(t, p)
			if proj.Name != name {
				t.Fatalf("projection name = %q, want %q", proj.Name, name)
			}
			if proj.SupportedTransports == nil || proj.Capabilities == nil {
				t.Fatalf("projection %q has nil required arrays", name)
			}
			for _, action := range proj.Actions {
				if action.Method == "" || action.Risk == "" {
					t.Fatalf("action %q did not resolve route method/risk", action.ID)
				}
				if (action.Open == plugin.OpenDock || action.Open == plugin.OpenDialog) && action.Panel == "" {
					t.Fatalf("action %q opens %q without a panel", action.ID, action.Open)
				}
				if action.Risk == plugin.RiskDestructive && !action.RequiresConfirm {
					t.Fatalf("destructive action %q must require confirmation", action.ID)
				}
			}
			if findings := pluginux.Errors(pluginux.Lint(p.Manifest(), p.Routes())); len(findings) > 0 {
				for _, finding := range findings {
					t.Errorf("%s: %s", finding.Path, finding.Message)
				}
				t.Fatalf("projection %q has plugin UX errors", name)
			}
			if _, err := json.Marshal(proj); err != nil {
				t.Fatalf("projection does not marshal: %v", err)
			}
		})
	}
}
