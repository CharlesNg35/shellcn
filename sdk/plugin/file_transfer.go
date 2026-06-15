package plugin

// FileTransferRequest is sent by the renderer to a StreamFileTransfer handler.
type FileTransferRequest struct {
	Type        string            `json:"type"`
	TransferID  string            `json:"transferId,omitempty"`
	Operation   string            `json:"operation,omitempty"`
	Paths       []string          `json:"paths,omitempty"`
	Destination string            `json:"destination,omitempty"`
	Overwrite   bool              `json:"overwrite,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

const (
	FileTransferRequestStart  = "start"
	FileTransferRequestCancel = "cancel"
)

// FileTransferFrame is emitted by a StreamFileTransfer handler to report lifecycle,
// progress, logs, completion, and failures.
type FileTransferFrame struct {
	Type        string            `json:"type"`
	TransferID  string            `json:"transferId,omitempty"`
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
	FileTransferFrameStatus   = "status"
	FileTransferFrameProgress = "progress"
	FileTransferFrameLog      = "log"
	FileTransferFrameComplete = "complete"
	FileTransferFrameError    = "error"
)
