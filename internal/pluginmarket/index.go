// Package pluginmarket fetches the plugin registry index and installs verified
// binaries: the indexed sha256 is the trust root for every download.
package pluginmarket

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Asset is one platform binary of an indexed version.
type Asset struct {
	SHA256 string   `json:"sha256"`
	URLs   []string `json:"urls"` // mirror first, upstream second
}

// Version is one installable plugin version from the index.
type Version struct {
	Version         string           `json:"version"`
	SDK             string           `json:"sdk"`
	APIVersion      int              `json:"apiVersion"`
	ProtocolVersion int              `json:"protocolVersion"`
	Yanked          bool             `json:"yanked,omitempty"`
	Assets          map[string]Asset `json:"assets"`
	Icon            plugin.Icon      `json:"icon"`
	// Projection is registry-verified; the admin UI reviews it before install.
	Projection json.RawMessage `json:"projection,omitempty"`
}

// Entry is one plugin in the index.
type Entry struct {
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	Repo        string    `json:"repo"`
	Homepage    string    `json:"homepage,omitempty"`
	License     string    `json:"license"`
	Maintainers []string  `json:"maintainers"`
	Versions    []Version `json:"versions"`
}

// Index is the registry document.
type Index struct {
	SchemaVersion int     `json:"schemaVersion"`
	GeneratedBy   string  `json:"generatedBy"`
	Plugins       []Entry `json:"plugins"`
}

func hostPlatform() string { return runtime.GOOS + "/" + runtime.GOARCH }

// Installable returns the newest runnable version (versions are ordered newest
// first in the index).
func Installable(e Entry) (Version, bool) {
	for _, v := range e.Versions {
		if v.Yanked || v.ProtocolVersion != grpcplugin.ProtocolVersion {
			continue
		}
		if _, ok := v.Assets[hostPlatform()]; ok {
			return v, true
		}
	}
	return Version{}, false
}

func FindVersion(e Entry, version string) (Version, error) {
	for _, v := range e.Versions {
		if v.Version != version {
			continue
		}
		if v.Yanked {
			return Version{}, fmt.Errorf("%w: version %s is yanked", plugin.ErrInvalidInput, version)
		}
		if v.ProtocolVersion != grpcplugin.ProtocolVersion {
			return Version{}, fmt.Errorf("%w: version %s targets wire protocol %d (gateway speaks %d)",
				plugin.ErrInvalidInput, version, v.ProtocolVersion, grpcplugin.ProtocolVersion)
		}
		if _, ok := v.Assets[hostPlatform()]; !ok {
			return Version{}, fmt.Errorf("%w: version %s has no %s build", plugin.ErrInvalidInput, version, hostPlatform())
		}
		return v, nil
	}
	return Version{}, fmt.Errorf("%w: version %s", plugin.ErrNotFound, version)
}
