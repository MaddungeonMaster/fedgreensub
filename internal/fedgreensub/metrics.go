package fedgreensub

import (
	"context"
	"runtime"
	"runtime/metrics"
	"time"
)

// SystemSample contains the host/runtime measurements used to populate the
// system portion of RuntimeMetrics.
type SystemSample struct {
	Timestamp  time.Time
	CPU        float64
	MemoryMB   float64
	Goroutines int
}

// GossipSample contains the gossip-layer counters used to populate the network
// and message-delivery portions of RuntimeMetrics.
type GossipSample struct {
	Timestamp time.Time

	BandwidthInBytes  uint64
	BandwidthOutBytes uint64

	IncomingMessages    uint64
	OutgoingMessages    uint64
	DuplicateMessages   uint64
	DroppedMessages     uint64
	SuccessfulPublishes uint64

	MeshDegree        int
	PeerCount         int
	HeartbeatDuration time.Duration
	PublishLatency    time.Duration
	PeerScoreAverage  float64
}

// SystemProbe produces host/runtime samples.
type SystemProbe interface {
	Snapshot(context.Context) (SystemSample, error)
}

// GossipProbe produces gossip-layer counters.
type GossipProbe interface {
	Snapshot(context.Context) (GossipSample, error)
}

// RuntimeSystemProbe is the default sampler backed by the Go runtime.
type RuntimeSystemProbe struct {
	lastCPUSeconds float64
	lastSampleTime time.Time
}

// NewRuntimeSystemProbe creates a runtime-backed system probe.
func NewRuntimeSystemProbe() *RuntimeSystemProbe {
	return &RuntimeSystemProbe{}
}

// Snapshot reads the current runtime metrics and derives a process CPU
// utilization estimate from the delta between samples.
func (p *RuntimeSystemProbe) Snapshot(context.Context) (SystemSample, error) {
	now := time.Now()
	cpuSeconds := readFloat64Metric("/cpu/classes/total:cpu-seconds")
	memoryMB := readMemoryMB()
	goroutines := runtime.NumGoroutine()

	cpuPercent := 0.0
	if !p.lastSampleTime.IsZero() {
		elapsedSeconds := now.Sub(p.lastSampleTime).Seconds()
		if elapsedSeconds > 0 && cpuSeconds >= p.lastCPUSeconds {
			cpuPercent = (cpuSeconds - p.lastCPUSeconds) / elapsedSeconds
			cpuPercent = cpuPercent / float64(max(runtime.NumCPU(), 1)) * 100
		}
	}

	p.lastSampleTime = now
	p.lastCPUSeconds = cpuSeconds

	return SystemSample{
		Timestamp:  now,
		CPU:        clampFloat64(cpuPercent, 0, 100),
		MemoryMB:   memoryMB,
		Goroutines: goroutines,
	}, nil
}

func readFloat64Metric(name string) float64 {
	samples := []metrics.Sample{{Name: name}}
	metrics.Read(samples)
	if len(samples) == 0 || samples[0].Value.Kind() != metrics.KindFloat64 {
		return 0
	}
	return samples[0].Value.Float64()
}

func readMemoryMB() float64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return float64(stats.Alloc) / (1024 * 1024)
}

func clampFloat64(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
