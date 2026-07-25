package fedgreensub

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MetricsCollector is the extension point that phase 2 will turn into a live
// sampler. The package keeps the interface small so it can be injected into the
// optimizer and runtime integration layers without pulling in concrete libp2p
// internals.
type MetricsCollector interface {
	Collect(context.Context) (RuntimeMetrics, error)
}

// Collector combines a system probe with a gossip probe and converts the raw
// samples into the normalized RuntimeMetrics structure used by the rest of the
// package.
type Collector struct {
	mu sync.Mutex

	systemProbe SystemProbe
	gossipProbe GossipProbe
	now         func() time.Time

	initialized bool
	lastSample  RuntimeMetrics

	lastGossip GossipSample
}

// CollectorOption configures a Collector.
type CollectorOption func(*Collector)

// NewCollector creates a metrics collector that defaults to the Go runtime for
// system metrics and accepts an optional gossip probe for libp2p integration.
func NewCollector(opts ...CollectorOption) *Collector {
	c := &Collector{
		systemProbe: NewRuntimeSystemProbe(),
		now:         time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.systemProbe == nil {
		c.systemProbe = NewRuntimeSystemProbe()
	}
	return c
}

// WithSystemProbe injects a custom system sampler.
func WithSystemProbe(probe SystemProbe) CollectorOption {
	return func(c *Collector) {
		c.systemProbe = probe
	}
}

// WithGossipProbe injects a gossip sampler.
func WithGossipProbe(probe GossipProbe) CollectorOption {
	return func(c *Collector) {
		c.gossipProbe = probe
	}
}

// WithClock injects a deterministic clock for tests.
func WithClock(now func() time.Time) CollectorOption {
	return func(c *Collector) {
		c.now = now
	}
}

// Collect gathers the latest system and gossip samples, converts counters into
// rates, and returns a fully populated RuntimeMetrics snapshot.
func (c *Collector) Collect(ctx context.Context) (RuntimeMetrics, error) {
	if c == nil {
		return RuntimeMetrics{}, errors.New("fedgreensub: nil collector")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	systemSample, err := c.collectSystem(ctx)
	if err != nil {
		return RuntimeMetrics{}, err
	}
	gossipSample, err := c.collectGossip(ctx)
	if err != nil {
		return RuntimeMetrics{}, err
	}

	metrics := RuntimeMetrics{
		Timestamp:           now,
		CPU:                 systemSample.CPU,
		MemoryMB:            systemSample.MemoryMB,
		Goroutines:          systemSample.Goroutines,
		MeshDegree:          gossipSample.MeshDegree,
		PeerCount:           gossipSample.PeerCount,
		PublishLatency:      durationToMillis(gossipSample.PublishLatency),
		HeartbeatDuration:   gossipSample.HeartbeatDuration,
		PeerScoreAverage:    gossipSample.PeerScoreAverage,
		BytesSent:           gossipSample.BandwidthOutBytes,
		BytesReceived:       gossipSample.BandwidthInBytes,
		DroppedMessages:     gossipSample.DroppedMessages,
		SuccessfulPublishes: gossipSample.SuccessfulPublishes,
	}

	if c.initialized {
		elapsedSeconds := now.Sub(c.lastSample.Timestamp).Seconds()
		if elapsedSeconds > 0 {
			metrics.IncomingRate = deltaRate(gossipSample.IncomingMessages, c.lastGossip.IncomingMessages, elapsedSeconds)
			metrics.OutgoingRate = deltaRate(gossipSample.OutgoingMessages, c.lastGossip.OutgoingMessages, elapsedSeconds)
			metrics.DuplicateRate = deltaRate(gossipSample.DuplicateMessages, c.lastGossip.DuplicateMessages, elapsedSeconds)
			metrics.BandwidthInBps = deltaBandwidth(gossipSample.BandwidthInBytes, c.lastGossip.BandwidthInBytes, elapsedSeconds)
			metrics.BandwidthOutBps = deltaBandwidth(gossipSample.BandwidthOutBytes, c.lastGossip.BandwidthOutBytes, elapsedSeconds)
			metrics.PeerUptimeSeconds = elapsedSeconds
		}
	}

	if metrics.HeartbeatDuration <= 0 {
		metrics.HeartbeatDuration = time.Duration(0)
	}
	if metrics.PublishLatency < 0 {
		metrics.PublishLatency = 0
	}
	if metrics.MeshDegree < 0 {
		metrics.MeshDegree = 0
	}
	if metrics.PeerCount < 0 {
		metrics.PeerCount = 0
	}

	c.initialized = true
	c.lastSample = metrics
	c.lastGossip = gossipSample

	return metrics, nil
}

func (c *Collector) collectSystem(ctx context.Context) (SystemSample, error) {
	if c.systemProbe == nil {
		return SystemSample{Timestamp: c.now()}, nil
	}
	return c.systemProbe.Snapshot(ctx)
}

func (c *Collector) collectGossip(ctx context.Context) (GossipSample, error) {
	if c.gossipProbe == nil {
		return GossipSample{Timestamp: c.now()}, nil
	}
	return c.gossipProbe.Snapshot(ctx)
}

func durationToMillis(duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	return float64(duration) / float64(time.Millisecond)
}

func deltaRate(current, previous uint64, elapsedSeconds float64) float64 {
	if current < previous || elapsedSeconds <= 0 {
		return 0
	}
	return float64(current-previous) / elapsedSeconds
}

func deltaBandwidth(current, previous uint64, elapsedSeconds float64) float64 {
	if current < previous || elapsedSeconds <= 0 {
		return 0
	}
	return float64(current-previous) * 8 / elapsedSeconds
}

// SnapshotStore keeps the latest metrics sample for consumers that should not
// block on live collection.
type SnapshotStore struct {
	mu      sync.RWMutex
	metrics RuntimeMetrics
}

func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{}
}

func (s *SnapshotStore) Update(metrics RuntimeMetrics) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.metrics = metrics
	s.mu.Unlock()
}

func (s *SnapshotStore) Latest() RuntimeMetrics {
	if s == nil {
		return RuntimeMetrics{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}
