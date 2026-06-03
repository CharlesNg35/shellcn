package plugin

import "slices"

// RecordingClass groups streams by recording format family.
type RecordingClass string

const (
	RecordingTerminal RecordingClass = "terminal"
	RecordingDesktop  RecordingClass = "desktop"
)

// RecordingFormat is a concrete on-disk recording encoding.
type RecordingFormat string

const (
	FormatAsciicastV2 RecordingFormat = "asciicast_v2"
	FormatWebMCanvas  RecordingFormat = "webm_canvas"
)

// RecordingPolicy is a connection's per-class recording setting. Off by default.
type RecordingPolicy string

const (
	PolicyDisabled RecordingPolicy = "disabled"
	PolicyManual   RecordingPolicy = "manual"
	PolicyAuto     RecordingPolicy = "auto"
)

// RecordingCapability is one recordable stream class a plugin declares.
type RecordingCapability struct {
	Class   RecordingClass
	Formats []RecordingFormat // ordered preference; Formats[0] is the default
	// StreamIDs are server-only: the projection never exposes the stream binding.
	StreamIDs     []string
	Authoritative bool
	InputCapture  bool
}

var (
	terminalFormats = map[RecordingFormat]bool{FormatAsciicastV2: true}
	desktopFormats  = map[RecordingFormat]bool{FormatWebMCanvas: true}
)

// DefaultFormat returns the capability's preferred (first) format.
func (c RecordingCapability) DefaultFormat() RecordingFormat {
	if len(c.Formats) == 0 {
		return ""
	}
	return c.Formats[0]
}

// SupportsFormat reports whether the capability declares the given format.
func (c RecordingCapability) SupportsFormat(f RecordingFormat) bool {
	return slices.Contains(c.Formats, f)
}

// Recordable reports whether a manifest declares any recording capability.
func (m Manifest) Recordable() bool { return len(m.Recording) > 0 }

// RecordingClassFor returns the capability covering a stream id, if any.
func (m Manifest) RecordingClassFor(streamID string) (RecordingCapability, bool) {
	for _, c := range m.Recording {
		if slices.Contains(c.StreamIDs, streamID) {
			return c, true
		}
	}
	return RecordingCapability{}, false
}

// SupportsRecordingClass reports whether the manifest declares the given class.
func (m Manifest) SupportsRecordingClass(class RecordingClass) bool {
	for _, c := range m.Recording {
		if c.Class == class {
			return true
		}
	}
	return false
}

// ValidRecordingPolicy reports whether p is a known connection recording policy.
func ValidRecordingPolicy(p RecordingPolicy) bool {
	switch p {
	case PolicyDisabled, PolicyManual, PolicyAuto:
		return true
	default:
		return false
	}
}
