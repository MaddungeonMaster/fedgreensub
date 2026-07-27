package fedgreensub

import (
	"sync"
	"time"
)

// EnergyWeights controls how each normalized metric contributes to the energy
// estimate.
type EnergyWeights struct {
	CPU               float64
	Bandwidth         float64
	Memory            float64
	DuplicateMessages float64
	HeartbeatCost     float64
}

// DefaultEnergyWeights returns the starting coefficients requested by the
// prototype design.
func DefaultEnergyWeights() EnergyWeights {
	return EnergyWeights{
		CPU:               0.35,
		Bandwidth:         0.25,
		Memory:            0.15,
		DuplicateMessages: 0.15,
		HeartbeatCost:     0.10,
	}
}

// EnergyEstimator computes a configurable score that proxies runtime power
// usage.
type EnergyEstimator struct {
	mu      sync.RWMutex
	weights EnergyWeights
}

// NewEnergyEstimator constructs a new estimator with the provided weights.
func NewEnergyEstimator(weights EnergyWeights) *EnergyEstimator {
	if weights == (EnergyWeights{}) {
		weights = DefaultEnergyWeights()
	}
	return &EnergyEstimator{weights: weights}
}

// SetWeights updates the estimator coefficients safely.
func (e *EnergyEstimator) SetWeights(weights EnergyWeights) {
	if e == nil {
		return
	}
	if weights == (EnergyWeights{}) {
		weights = DefaultEnergyWeights()
	}
	e.mu.Lock()
	e.weights = weights
	e.mu.Unlock()
}

// Weights returns a copy of the current estimator coefficients.
func (e *EnergyEstimator) Weights() EnergyWeights {
	if e == nil {
		return DefaultEnergyWeights()
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.weights
}

// EstimateEnergy returns a normalized score where larger values indicate a
// more expensive runtime profile.
func EstimateEnergy(metrics RuntimeMetrics) float64 {
	return EstimateEnergyWithWeights(metrics, DefaultEnergyWeights())
}

// EstimateEnergyWithWeights computes the energy score using explicit custom
// coefficients.
func EstimateEnergyWithWeights(metrics RuntimeMetrics, weights EnergyWeights) float64 {
	return NewEnergyEstimator(weights).EstimateEnergy(metrics)
}

// EstimateEnergy returns the weighted score for a configured estimator.
func (e *EnergyEstimator) EstimateEnergy(metrics RuntimeMetrics) float64 {
	if e == nil {
		e = NewEnergyEstimator(DefaultEnergyWeights())
	}
	weights := e.Weights()

	return weights.CPU*normalizePercentage(metrics.CPU) +
		weights.Bandwidth*normalizeBandwidth(metrics.BandwidthInBps, metrics.BandwidthOutBps) +
		weights.Memory*normalizeMemory(metrics.MemoryMB) +
		weights.DuplicateMessages*normalizeRate(metrics.DuplicateRate) +
		weights.HeartbeatCost*normalizeHeartbeat(metrics.HeartbeatDuration)
}

func normalizePercentage(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 1
	}
	return value / 100
}

func normalizeBandwidth(inBps, outBps float64) float64 {
	bandwidth := inBps + outBps
	if bandwidth < 0 {
		return 0
	}
	return bandwidth / (100 * 1024 * 1024)
}

func normalizeMemory(memoryMB float64) float64 {
	if memoryMB < 0 {
		return 0
	}
	return memoryMB / 4096
}

func normalizeRate(rate float64) float64 {
	if rate < 0 {
		return 0
	}
	if rate > 1 {
		return 1
	}
	return rate
}

func normalizeHeartbeat(duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	// Duration is measured in nanoseconds; normalize against one second.
	return float64(duration) / float64(time.Second)
}
