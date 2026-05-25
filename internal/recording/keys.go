package recording

import (
	"time"

	"github.com/charlesng/shellcn/internal/plugin"
)

// fileExt maps a recording format to its blob file extension.
func fileExt(format plugin.RecordingFormat) string {
	switch format {
	case plugin.FormatAsciicastV2:
		return ".cast"
	case plugin.FormatWebMCanvas:
		return ".webm"
	default:
		return ".bin"
	}
}

// StorageKey builds the blob key for a recording, namespaced by connection.
func StorageKey(connectionID, recordingID string, format plugin.RecordingFormat) string {
	return connectionID + "/" + recordingID + fileExt(format)
}

// ContentType is the HTTP content type a recording's bytes are served with.
func ContentType(format plugin.RecordingFormat) string {
	switch format {
	case plugin.FormatAsciicastV2:
		return "application/x-asciicast"
	case plugin.FormatWebMCanvas:
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

// ExpiryFor computes a recording's expiry from per-connection and default
// retention. Per-connection wins; nil means keep indefinitely (retention off).
func ExpiryFor(start time.Time, connectionDays, defaultDays int) *time.Time {
	days := connectionDays
	if days <= 0 {
		days = defaultDays
	}
	if days <= 0 {
		return nil
	}
	t := start.AddDate(0, 0, days)
	return &t
}
