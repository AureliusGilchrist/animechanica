package completion_monitor

import (
	"sync"
	"time"
)

// Options defines runtime parameters for the Completion Monitor.
type Options struct {
	PollInterval           time.Duration // e.g., 30s
	ResumePerTick          int           // e.g., 20
	HydrationWorkers       int           // e.g., 3
	HydrationQueueSize     int           // e.g., 500
	DirDebounce            time.Duration // e.g., 30s
	ResumeDebounce         time.Duration // e.g., 120s
	StatusEmitInterval     time.Duration // e.g., 10s
	ProcessedLRUSize       int           // e.g., 50000
}

// Status provides a snapshot of the monitor state suitable for APIs/WS.
type Status struct {
	Running           bool      `json:"running"`
	LastTick          time.Time `json:"lastTick"`
	UnfinishedCount   int       `json:"unfinishedCount"`
	ResumedCount      int       `json:"resumedCount"`
	FinishedDetected  int       `json:"finishedDetected"`
	HydrationsQueued  int       `json:"hydrationsQueued"`
	ProcessedTotal    int       `json:"processedTotal"`
	LastError         string    `json:"lastError,omitempty"`
}

// Monitor is a lightweight scaffold; full logic will be implemented incrementally.
type Monitor struct {
	mu      sync.RWMutex
	opts    Options
	status  Status
	running bool
	stopCh  chan struct{}
}

// New creates a monitor with safe defaults if options are zeroed.
func New(opts Options) *Monitor {
	// Apply defaults
	if opts.PollInterval == 0 {
		opts.PollInterval = 30 * time.Second
	}
	if opts.ResumePerTick <= 0 {
		opts.ResumePerTick = 20
	}
	if opts.HydrationWorkers <= 0 {
		opts.HydrationWorkers = 3
	}
	if opts.HydrationQueueSize <= 0 {
		opts.HydrationQueueSize = 500
	}
	if opts.DirDebounce == 0 {
		opts.DirDebounce = 30 * time.Second
	}
	if opts.ResumeDebounce == 0 {
		opts.ResumeDebounce = 120 * time.Second
	}
	if opts.StatusEmitInterval == 0 {
		opts.StatusEmitInterval = 10 * time.Second
	}
	if opts.ProcessedLRUSize <= 0 {
		opts.ProcessedLRUSize = 50000
	}

	return &Monitor{
		opts:   opts,
		stopCh: make(chan struct{}),
		status: Status{},
	}
}

// Start begins the background loop (no-op scaffold for now).
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return
	}
	m.running = true
	m.status.Running = true
	m.status.LastTick = time.Now()
}

// Stop halts the background loop.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	close(m.stopCh)
	m.running = false
	m.status.Running = false
}

// Running reports whether the monitor is active.
func (m *Monitor) Running() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns a copy of the current status.
func (m *Monitor) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}
