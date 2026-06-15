package plugin

// FileJobRequest is sent by the renderer to a StreamFileJob handler.
type FileJobRequest struct {
	Type        string            `json:"type"`
	JobID       string            `json:"jobId,omitempty"`
	Operation   string            `json:"operation,omitempty"`
	Paths       []string          `json:"paths,omitempty"`
	Destination string            `json:"destination,omitempty"`
	Overwrite   bool              `json:"overwrite,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

const (
	FileJobRequestStart  = "start"
	FileJobRequestCancel = "cancel"
)

// FileJobFrame is emitted by a StreamFileJob handler to report lifecycle,
// progress, logs, completion, and failures.
type FileJobFrame struct {
	Type        string            `json:"type"`
	JobID       string            `json:"jobId,omitempty"`
	Status      string            `json:"status,omitempty"`
	Message     string            `json:"message,omitempty"`
	Path        string            `json:"path,omitempty"`
	Operation   string            `json:"operation,omitempty"`
	Percent     *float64          `json:"percent,omitempty"`
	BytesDone   int64             `json:"bytesDone,omitempty"`
	BytesTotal  int64             `json:"bytesTotal,omitempty"`
	FilesDone   int               `json:"filesDone,omitempty"`
	FilesTotal  int               `json:"filesTotal,omitempty"`
	RateBps     int64             `json:"rateBps,omitempty"`
	DownloadURL string            `json:"downloadUrl,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
}

const (
	FileJobFrameStatus   = "status"
	FileJobFrameProgress = "progress"
	FileJobFrameLog      = "log"
	FileJobFrameComplete = "complete"
	FileJobFrameError    = "error"
)
