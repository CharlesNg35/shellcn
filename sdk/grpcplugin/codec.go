package grpcplugin

import (
	"encoding/json"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// manifestBundle is the GetManifest payload: the declarative manifest plus the
// route metadata the host needs to register, enforce, and route. Handler funcs
// stay on the plugin side and are replaced by gRPC shims on the host.
type manifestBundle struct {
	Manifest plugin.Manifest `json:"manifest"`
	Routes   []plugin.Route  `json:"routes"`
}

// EncodeManifest marshals a plugin's manifest and routes for the wire.
func EncodeManifest(m plugin.Manifest, routes []plugin.Route) ([]byte, error) {
	return json.Marshal(manifestBundle{Manifest: m, Routes: routes})
}

// DecodeManifest reverses EncodeManifest. The returned routes carry no handlers.
func DecodeManifest(data []byte) (plugin.Manifest, []plugin.Route, error) {
	var b manifestBundle
	if err := json.Unmarshal(data, &b); err != nil {
		return plugin.Manifest{}, nil, err
	}
	return b.Manifest, b.Routes, nil
}
