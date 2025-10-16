package monitoring

import (
	"sync"
	"sync/atomic"
	"time"
)

type statStore struct {
	authSuccess atomic.Uint64
	authFailure atomic.Uint64
	authError   atomic.Uint64

	permissionAllowed atomic.Uint64
	permissionDenied  atomic.Uint64
	permissionError   atomic.Uint64

	activeSessions       atomic.Int64
	sessionTotalDuration atomic.Uint64 // nanoseconds
	sessionCount         atomic.Uint64
	sessionLastDuration  atomic.Int64
	sessionLastEndedAt   atomic.Int64

	realtimeConnections atomic.Int64
	realtimeBroadcasts  atomic.Uint64
	realtimeFailures    atomic.Uint64
	realtimeLastFailure atomic.Value // *FailureRecord

	maintenance sync.Map // string -> *maintenanceStats
	protocols   sync.Map // string -> *protocolStats
	webVitals   sync.Map // string -> *webVitalStats
}

func newStatStore() *statStore {
	store := &statStore{}
	store.realtimeLastFailure.Store((*FailureRecord)(nil))
	return store
}

func (s *statStore) cloneMaintenance() []MaintenanceJobSummary {
	summaries := []MaintenanceJobSummary{}
	s.maintenance.Range(func(key, value any) bool {
		job := key.(string)
		stats := value.(*maintenanceStats)
		summaries = append(summaries, stats.snapshot(job))
		return true
	})
	return summaries
}

func (s *statStore) cloneProtocols() []ProtocolSummary {
	summaries := []ProtocolSummary{}
	s.protocols.Range(func(key, value any) bool {
		protocol := key.(string)
		stats := value.(*protocolStats)
		summaries = append(summaries, stats.snapshot(protocol))
		return true
	})
	return summaries
}

func (s *statStore) cloneWebVitals() []WebVitalSummary {
	vitals := []WebVitalSummary{}
	s.webVitals.Range(func(key, value any) bool {
		metric := key.(string)
		stats := value.(*webVitalStats)
		vitals = append(vitals, stats.snapshot(metric))
		return true
	})
	return vitals
}

func (s *statStore) summary() Summary {
	lastFailure, _ := s.realtimeLastFailure.Load().(*FailureRecord)
	totalDuration := s.sessionTotalDuration.Load()
	count := s.sessionCount.Load()
	var avgSeconds float64
	if count > 0 {
		avgSeconds = float64(totalDuration) / float64(count) / float64(time.Second)
	}

	lastDuration := time.Duration(s.sessionLastDuration.Load())
	lastEnded := time.Unix(0, s.sessionLastEndedAt.Load())

	return Summary{
		GeneratedAt: time.Now(),
		Auth: AuthSummary{
			Success: s.authSuccess.Load(),
			Failure: s.authFailure.Load(),
			Error:   s.authError.Load(),
		},
		Permissions: PermissionSummary{
			Allowed: s.permissionAllowed.Load(),
			Denied:  s.permissionDenied.Load(),
			Error:   s.permissionError.Load(),
		},
		Sessions: SessionSummary{
			Active:                 s.activeSessions.Load(),
			Completed:              count,
			AverageDurationSeconds: avgSeconds,
			LastDuration:           lastDuration,
			LastEndedAt:            lastEnded,
		},
		Realtime: RealtimeSummary{
			ActiveConnections: s.realtimeConnections.Load(),
			Broadcasts:        s.realtimeBroadcasts.Load(),
			Failures:          s.realtimeFailures.Load(),
			LastFailure:       lastFailure,
		},
		Maintenance: MaintenanceSummary{
			Jobs: s.cloneMaintenance(),
		},
		Protocols: s.cloneProtocols(),
		WebVitals: s.cloneWebVitals(),
	}
}

func (s *statStore) recordAuth(result string) {
	switch result {
	case "success":
		s.authSuccess.Add(1)
	case "failure":
		s.authFailure.Add(1)
	default:
		s.authError.Add(1)
	}
}

func (s *statStore) recordPermission(result string) {
	switch result {
	case "allowed":
		s.permissionAllowed.Add(1)
	case "denied":
		s.permissionDenied.Add(1)
	default:
		s.permissionError.Add(1)
	}
}

func (s *statStore) adjustActiveSessions(delta int64) {
	newValue := s.activeSessions.Add(delta)
	if newValue < 0 {
		s.activeSessions.Store(0)
	}
}

func (s *statStore) recordSessionDuration(d time.Duration) {
	if d < 0 {
		d = 0
	}
	s.sessionTotalDuration.Add(uint64(d))
	s.sessionCount.Add(1)
	s.sessionLastDuration.Store(int64(d))
	s.sessionLastEndedAt.Store(time.Now().UnixNano())
}

func (s *statStore) recordRealtimeConnection(delta int64) {
	newValue := s.realtimeConnections.Add(delta)
	if newValue < 0 {
		s.realtimeConnections.Store(0)
	}
}

func (s *statStore) recordRealtimeBroadcast(stream string) {
	s.realtimeBroadcasts.Add(1)
}

func (s *statStore) recordRealtimeFailure(record FailureRecord) {
	s.realtimeFailures.Add(1)
	cloned := record
	s.realtimeLastFailure.Store(&cloned)
}

func (s *statStore) maintenanceEntry(job string) *maintenanceStats {
	value, ok := s.maintenance.Load(job)
	if ok {
		return value.(*maintenanceStats)
	}
	stats := &maintenanceStats{}
	actual, _ := s.maintenance.LoadOrStore(job, stats)
	return actual.(*maintenanceStats)
}

func (s *statStore) protocolEntry(protocol string) *protocolStats {
	value, ok := s.protocols.Load(protocol)
	if ok {
		return value.(*protocolStats)
	}
	stats := &protocolStats{}
	actual, _ := s.protocols.LoadOrStore(protocol, stats)
	return actual.(*protocolStats)
}

func (s *statStore) recordWebVital(metric, rating string, value float64) {
	entry := s.webVitalEntry(metric)
	entry.record(rating, value)
}

func (s *statStore) webVitalEntry(metric string) *webVitalStats {
	value, ok := s.webVitals.Load(metric)
	if ok {
		return value.(*webVitalStats)
	}
	stats := &webVitalStats{}
	actual, _ := s.webVitals.LoadOrStore(metric, stats)
	return actual.(*webVitalStats)
}

type webVitalStats struct {
	mu          sync.Mutex
	total       float64
	count       uint64
	lastValue   float64
	lastRating  string
	lastUpdated time.Time
}

func (w *webVitalStats) record(rating string, value float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if value < 0 {
		value = 0
	}
	w.total += value
	w.count++
	w.lastValue = value
	w.lastRating = rating
	w.lastUpdated = time.Now()
}

func (w *webVitalStats) snapshot(metric string) WebVitalSummary {
	w.mu.Lock()
	defer w.mu.Unlock()
	var average float64
	if w.count > 0 {
		average = w.total / float64(w.count)
	}
	return WebVitalSummary{
		Metric:         metric,
		LastValue:      w.lastValue,
		AverageValue:   average,
		Samples:        w.count,
		LastRecordedAt: w.lastUpdated,
		LastRating:     w.lastRating,
	}
}

type maintenanceStats struct {
	lastStatus           atomic.Value // string
	lastError            atomic.Value // string
	lastRun              atomic.Int64 // unix nano
	lastDuration         atomic.Int64 // nanoseconds
	consecutiveFailures  atomic.Uint64
	totalRuns            atomic.Uint64
	lastSuccessfulRun    atomic.Int64
	consecutiveSuccesses atomic.Uint64
}

func (m *maintenanceStats) snapshot(job string) MaintenanceJobSummary {
	status, _ := m.lastStatus.Load().(string)
	errMsg, _ := m.lastError.Load().(string)
	lastRun := time.Unix(0, m.lastRun.Load())
	lastSuccess := time.Unix(0, m.lastSuccessfulRun.Load())

	return MaintenanceJobSummary{
		Job:                 job,
		LastStatus:          status,
		LastRunAt:           lastRun,
		LastDuration:        time.Duration(m.lastDuration.Load()),
		LastError:           errMsg,
		ConsecutiveFailures: m.consecutiveFailures.Load(),
		ConsecutiveSuccess:  m.consecutiveSuccesses.Load(),
		LastSuccessAt:       lastSuccess,
		TotalRuns:           m.totalRuns.Load(),
	}
}

func (m *maintenanceStats) record(result, message string, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}
	now := time.Now()
	m.lastStatus.Store(result)
	m.lastError.Store(message)
	m.lastRun.Store(now.UnixNano())
	m.lastDuration.Store(int64(duration))
	m.totalRuns.Add(1)

	switch result {
	case "success":
		m.consecutiveFailures.Store(0)
		m.consecutiveSuccesses.Add(1)
		m.lastSuccessfulRun.Store(now.UnixNano())
	default:
		m.consecutiveFailures.Add(1)
		m.consecutiveSuccesses.Store(0)
	}
}

type protocolStats struct {
	success        atomic.Uint64
	failure        atomic.Uint64
	lastStatus     atomic.Value // string
	lastError      atomic.Value // string
	lastDuration   atomic.Int64
	lastCompleted  atomic.Int64
	totalLatencyNs atomic.Uint64
	totalLaunches  atomic.Uint64
}

func (p *protocolStats) snapshot(protocol string) ProtocolSummary {
	status, _ := p.lastStatus.Load().(string)
	errMsg, _ := p.lastError.Load().(string)
	total := p.totalLaunches.Load()
	totalLatency := p.totalLatencyNs.Load()

	var avg float64
	if total > 0 {
		avg = float64(totalLatency) / float64(total) / float64(time.Second)
	}

	return ProtocolSummary{
		Protocol:              protocol,
		Success:               p.success.Load(),
		Failure:               p.failure.Load(),
		LastStatus:            status,
		LastDuration:          time.Duration(p.lastDuration.Load()),
		LastCompletedAt:       time.Unix(0, p.lastCompleted.Load()),
		LastError:             errMsg,
		AverageLatencySeconds: avg,
	}
}

func (p *protocolStats) record(result, message string, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}
	now := time.Now()
	switch result {
	case "success":
		p.success.Add(1)
	case "failure":
		p.failure.Add(1)
	default:
		// treat unknown as failure for counters
		p.failure.Add(1)
	}

	p.lastStatus.Store(result)
	p.lastError.Store(message)
	p.lastDuration.Store(int64(duration))
	p.lastCompleted.Store(now.UnixNano())
	p.totalLaunches.Add(1)
	p.totalLatencyNs.Add(uint64(duration))
}
