package fedgreensub

import "time"

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
	weights EnergyWeights
}

// NewEnergyEstimator constructs a new estimator with the provided weights.
func NewEnergyEstimator(weights EnergyWeights) *EnergyEstimator {
	if weights == (EnergyWeights{}) {
		weights = DefaultEnergyWeights()
	}
	return &EnergyEstimator{weights: weights}
}

// EstimateEnergy returns a normalized score where larger values indicate a
// more expensive runtime profile.
func EstimateEnergy(metrics RuntimeMetrics) float64 {
	return NewEnergyEstimator(DefaultEnergyWeights()).EstimateEnergy(metrics)
}

// EstimateEnergy returns the weighted score for a configured estimator.
func (e *EnergyEstimator) EstimateEnergy(metrics RuntimeMetrics) float64 {
	if e == nil {
		e = NewEnergyEstimator(DefaultEnergyWeights())
	}

	return e.weights.CPU*normalizePercentage(metrics.CPU) +
		e.weights.Bandwidth*normalizeBandwidth(metrics.BandwidthInBps, metrics.BandwidthOutBps) +
		e.weights.Memory*normalizeMemory(metrics.MemoryMB) +
		e.weights.DuplicateMessages*normalizeRate(metrics.DuplicateRate) +
		e.weights.HeartbeatCost*normalizeHeartbeat(metrics.HeartbeatDuration)
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
