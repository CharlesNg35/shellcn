package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func allTestPlugins(t testing.TB) []plugin.Plugin {
	t.Helper()
	plugins := all()
	for _, p := range plugins {
		plugintest.ValidatePlugin(t, p)
	}
	return plugins
}

func testPlugin(t testing.TB, name string) plugin.Plugin {
	t.Helper()
	for _, p := range allTestPlugins(t) {
		if p.Manifest().Name == name {
			return p
		}
	}
	t.Fatalf("plugin %q was not found", name)
	return nil
}

func testManifest(t testing.TB, name string) plugin.Manifest {
	t.Helper()
	return testPlugin(t, name).Manifest()
}

func testProjection(t testing.TB, name string) plugin.Projection {
	t.Helper()
	return plugintest.Projection(t, testPlugin(t, name))
}
