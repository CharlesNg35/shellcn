package recording

// Metrics is the subset of telemetry the recording engine reports. It is
// satisfied by *telemetry.Metrics; a nil engine metrics falls back to noopMetrics.
type Metrics interface {
	RecordingStarted()
	RecordingFinished()
	AddRecordingBytes(n int)
	RecordingFailed()
}

type noopMetrics struct{}

func (noopMetrics) RecordingStarted()     {}
func (noopMetrics) RecordingFinished()    {}
func (noopMetrics) AddRecordingBytes(int) {}
func (noopMetrics) RecordingFailed()      {}
