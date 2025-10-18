package handlers

import (
	"context"
	"strings"

	"github.com/charlesng35/shellcn/internal/drivers"
)

func workspaceDescriptorID(protocolID string) string {
	if protocolID == "" {
		return "workspace/unknown"
	}
	return "workspace/" + strings.ToLower(strings.TrimSpace(protocolID))
}

func driverCapabilitiesMap(reg *drivers.Registry, protocolID string, sftpEnabled bool) (map[string]any, error) {
	if reg == nil {
		return nil, nil
	}

	caps, err := reg.Capabilities(context.Background(), protocolID)
	if err != nil {
		return nil, err
	}

	features := map[string]bool{
		"terminal":           caps.Terminal,
		"desktop":            caps.Desktop,
		"file_transfer":      caps.FileTransfer,
		"clipboard":          caps.Clipboard,
		"session_recording":  caps.SessionRecording,
		"metrics":            caps.Metrics,
		"reconnect":          caps.Reconnect,
		"supports_sftp":      sftpEnabled,
		"supports_recording": caps.SessionRecording,
	}
	for key, value := range caps.Extras {
		features[key] = value
	}

	panes := []string{}
	if caps.Terminal {
		panes = append(panes, "terminal")
	}
	if sftpEnabled || caps.FileTransfer {
		panes = append(panes, "files")
	}

	return map[string]any{
		"features": features,
		"panes":    panes,
	}, nil
}
