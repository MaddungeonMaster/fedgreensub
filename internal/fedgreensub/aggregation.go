package fedgreensub

import (
	"fmt"
	"math"
)

type Aggregator interface {
	FedAvg([]ModelState) (ModelState, error)
	EnergyWeightedFedAvg([]ModelState, []float64) (ModelState, error)
	TrustWeightedFedAvg([]ModelState, []float64, float64) (ModelState, error)
}
type AggregationResult struct {
	Round        uint64
	Aggregated   ModelState
	Contributors int
	EnergyAware  bool
}
type AggregatorImpl struct{}

func NewAggregator() *AggregatorImpl { return &AggregatorImpl{} }

func (a *AggregatorImpl) FedAvg(states []ModelState) (ModelState, error) {
	if err := validateAggregationStates(states); err != nil {
		return ModelState{}, err
	}
	if len(states) == 0 {
		return ModelState{}, nil
	}
	weights := make([]float64, len(states))
	for i, s := range states {
		weights[i] = float64(s.Samples)
	}
	return mergeWeighted(states, weights)
}

// EnergyWeightedFedAvg uses a normalized, modelled resource-availability
// score in [0,1], not a physical energy measurement. Its contribution is
// sample count times resource availability times observed connectivity.
func (a *AggregatorImpl) EnergyWeightedFedAvg(states []ModelState, resources []float64) (ModelState, error) {
	if len(states) != len(resources) {
		return ModelState{}, fmt.Errorf("mismatched state and resource score counts: %d != %d", len(states), len(resources))
	}
	if err := validateAggregationStates(states); err != nil {
		return ModelState{}, err
	}
	if len(states) == 0 {
		return ModelState{}, nil
	}
	weights := make([]float64, len(states))
	for i, s := range states {
		if resources[i] < 0 || resources[i] > 1 || math.IsNaN(resources[i]) || math.IsInf(resources[i], 0) {
			return ModelState{}, fmt.Errorf("invalid modelled resource score at index %d", i)
		}
		weights[i] = float64(s.Samples) * resources[i] * ConnectivityScore(s.PacketLossRate, s.PeerUptimeSeconds, s.SuccessfulPublishes)
	}
	return mergeWeighted(states, weights)
}

// TrustWeightedFedAvg uses capped sample count × connectivity × trust factor.
// The trust factor is 0.1 + 0.9*effectiveTrust; a participant's sample count
// is capped at 1000 before weighting, so it cannot alone dominate a round.
// Missing/new trust is neutral (0.5); trustWeight scales trust's influence.
func (a *AggregatorImpl) TrustWeightedFedAvg(states []ModelState, trustScores []float64, trustWeight float64) (ModelState, error) {
	if len(states) != len(trustScores) {
		return ModelState{}, fmt.Errorf("mismatched state and trust score counts: %d != %d", len(states), len(trustScores))
	}
	if err := validateAggregationStates(states); err != nil {
		return ModelState{}, err
	}
	if len(states) == 0 {
		return ModelState{}, nil
	}
	if math.IsNaN(trustWeight) || math.IsInf(trustWeight, 0) || trustWeight < 0 {
		return ModelState{}, fmt.Errorf("invalid trust weight")
	}
	if trustWeight > 1 {
		trustWeight = 1
	}
	weights := make([]float64, len(states))
	for i, s := range states {
		trust := trustScores[i]
		if math.IsNaN(trust) || math.IsInf(trust, 0) {
			return ModelState{}, fmt.Errorf("invalid trust score at index %d", i)
		}
		trust = clamp01(trust)
		effective := .5 + trustWeight*(trust-.5)
		weights[i] = float64(minInt64(s.Samples, 1000)) * (.1 + .9*effective) * ConnectivityScore(s.PacketLossRate, s.PeerUptimeSeconds, s.SuccessfulPublishes)
	}
	return mergeWeighted(states, weights)
}

func mergeWeighted(states []ModelState, weights []float64) (ModelState, error) {
	if len(states) != len(weights) {
		return ModelState{}, fmt.Errorf("mismatched state and aggregation weight counts")
	}
	total := 0.0
	for i, w := range weights {
		if w < 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			return ModelState{}, fmt.Errorf("invalid aggregation weight at index %d", i)
		}
		total += w
	}
	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return ModelState{}, fmt.Errorf("aggregation has zero or invalid total weight")
	}
	merged := ModelState{Weights: make([]float64, len(states[0].Weights)), Biases: make([]float64, len(states[0].Biases)), FeatureVersion: states[0].FeatureVersion}
	for i := range merged.Weights {
		for j, s := range states {
			merged.Weights[i] += weights[j] * s.Weights[i]
		}
		merged.Weights[i] /= total
	}
	for i := range merged.Biases {
		for j, s := range states {
			merged.Biases[i] += weights[j] * s.Biases[i]
		}
		merged.Biases[i] /= total
	}
	merged.Samples = totalSamples(states)
	return merged, nil
}
func validateAggregationStates(states []ModelState) error {
	if len(states) == 0 {
		return nil
	}
	weights, biases, version := len(states[0].Weights), len(states[0].Biases), states[0].FeatureVersion
	for _, s := range states {
		if len(s.Weights) != weights || len(s.Biases) != biases {
			return fmt.Errorf("model parameter dimensions do not match")
		}
		if s.FeatureVersion != 0 && version != 0 && s.FeatureVersion != version {
			return fmt.Errorf("model feature versions do not match")
		}
		if s.Samples < 0 {
			return fmt.Errorf("model has negative sample count")
		}
		if math.IsNaN(s.PacketLossRate) || math.IsInf(s.PacketLossRate, 0) || math.IsNaN(s.PeerUptimeSeconds) || math.IsInf(s.PeerUptimeSeconds, 0) {
			return fmt.Errorf("model has invalid connectivity metadata")
		}
		if err := ValidateModelState(s, 0, 0); err != nil {
			return err
		}
	}
	return nil
}
func minInt64(value, maximum int64) int64 {
	if value > maximum {
		return maximum
	}
	return value
}
func totalSamples(states []ModelState) int64 {
	var total int64
	for _, s := range states {
		total += s.Samples
	}
	return total
}
func ConnectivityScore(packetLoss, uptime float64, publishes uint64) float64 {
	if packetLoss < 0 {
		packetLoss = 0
	}
	if packetLoss > 1 {
		packetLoss = 1
	}
	if uptime < 0 {
		uptime = 0
	}
	if publishes == 0 {
		return .5 * (1 - packetLoss)
	}
	base := 1 - packetLoss
	if uptime > 0 {
		base *= 1 - (1 / (1 + uptime/600))
	}
	base *= 1 - (1 / (1 + float64(publishes)/100))
	if base < .01 {
		return .01
	}
	if base > 1 {
		return 1
	}
	return base
}
