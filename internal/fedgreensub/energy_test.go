package fedgreensub

import (
	"math"
	"testing"
	"time"
)

func TestEstimateEnergyUsesDefaultWeights(t *testing.T) {
	metrics := RuntimeMetrics{
		CPU:               50,
		BandwidthInBps:    20 * 1024 * 1024,
		BandwidthOutBps:   20 * 1024 * 1024,
		MemoryMB:          2048,
		DuplicateRate:     0.4,
		HeartbeatDuration: 500 * time.Millisecond,
	}

	got := EstimateEnergy(metrics)
	want := 0.35*0.5 + 0.25*0.4 + 0.15*0.5 + 0.15*0.4 + 0.10*0.5

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("EstimateEnergy() = %f, want %f", got, want)
	}
}

func TestEstimateEnergyWithWeights(t *testing.T) {
	metrics := RuntimeMetrics{CPU: 80}
	weights := EnergyWeights{CPU: 1}

	got := EstimateEnergyWithWeights(metrics, weights)
	want := 0.8

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("EstimateEnergyWithWeights() = %f, want %f", got, want)
	}
}

func TestEnergyEstimatorSetWeights(t *testing.T) {
	estimator := NewEnergyEstimator(DefaultEnergyWeights())
	estimator.SetWeights(EnergyWeights{CPU: 1})

	weights := estimator.Weights()
	if weights.CPU != 1 || weights.Bandwidth != 0 || weights.Memory != 0 || weights.DuplicateMessages != 0 || weights.HeartbeatCost != 0 {
		t.Fatalf("unexpected weights: %+v", weights)
	}
}

func TestEstimateEnergyClampsNegativeInputs(t *testing.T) {
	metrics := RuntimeMetrics{
		CPU:               -10,
		BandwidthInBps:    -1,
		BandwidthOutBps:   -1,
		MemoryMB:          -1,
		DuplicateRate:     -1,
		HeartbeatDuration: -time.Second,
	}

	if got := EstimateEnergy(metrics); got != 0 {
		t.Fatalf("EstimateEnergy() = %f, want 0", got)
	}
}
