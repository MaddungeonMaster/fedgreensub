package fedgreensub

import "fmt"

// Aggregator combines model updates from multiple peers.
type Aggregator interface {
	FedAvg([]ModelState) (ModelState, error)
	EnergyWeightedFedAvg([]ModelState, []float64) (ModelState, error)
}

// AggregationResult records the outcome of a federated round.
type AggregationResult struct {
	Round        uint64
	Aggregated   ModelState
	Contributors int
	EnergyAware  bool
}

// AggregatorImpl is the default implementation of the federated averaging
// strategy for FedGreenSub.
type AggregatorImpl struct{}

func NewAggregator() *AggregatorImpl {
	return &AggregatorImpl{}
}

func (a *AggregatorImpl) FedAvg(states []ModelState) (ModelState, error) {
	if len(states) == 0 {
		return ModelState{}, nil
	}
	if len(states) == 1 {
		return cloneModelState(states[0]), nil
	}

	merged := ModelState{Weights: make([]float64, len(states[0].Weights)), Biases: make([]float64, len(states[0].Biases))}
	totalSamples := int64(0)
	for _, state := range states {
		totalSamples += state.Samples
	}
	if totalSamples == 0 {
		for i := range states[0].Weights {
			merged.Weights[i] = states[0].Weights[i]
		}
		for i := range states[0].Biases {
			merged.Biases[i] = states[0].Biases[i]
		}
		merged.Samples = totalSamples
		return merged, nil
	}

	for i := range merged.Weights {
		weightedSum := 0.0
		for _, state := range states {
			if i < len(state.Weights) {
				weightedSum += float64(state.Samples) * state.Weights[i]
			}
		}
		merged.Weights[i] = weightedSum / float64(totalSamples)
	}
	for i := range merged.Biases {
		weightedSum := 0.0
		for _, state := range states {
			if i < len(state.Biases) {
				weightedSum += float64(state.Samples) * state.Biases[i]
			}
		}
		merged.Biases[i] = weightedSum / float64(totalSamples)
	}
	merged.Samples = totalSamples
	return merged, nil
}

func (a *AggregatorImpl) EnergyWeightedFedAvg(states []ModelState, energyScores []float64) (ModelState, error) {
	if len(states) == 0 {
		return ModelState{}, nil
	}
	if len(states) != len(energyScores) {
		return ModelState{}, fmt.Errorf("mismatched state and energy score counts: %d != %d", len(states), len(energyScores))
	}
	if len(states) == 1 {
		return cloneModelState(states[0]), nil
	}

	weightTotal := 0.0
	weights := make([]float64, len(states))
	for i, state := range states {
		base := float64(state.Samples)
		batteryScore := 1.0
		if energyScores[i] > 0 {
			batteryScore = energyScores[i]
		}
		connectivity := ConnectivityScore(state.PacketLossRate, state.PeerUptimeSeconds, state.SuccessfulPublishes)
		w := base * batteryScore * connectivity
		weights[i] = w
		weightTotal += w
	}
	if weightTotal == 0 {
		return a.FedAvg(states)
	}

	merged := ModelState{Weights: make([]float64, len(states[0].Weights)), Biases: make([]float64, len(states[0].Biases))}
	for i := range merged.Weights {
		sum := 0.0
		for j, state := range states {
			if i < len(state.Weights) {
				sum += weights[j] * state.Weights[i]
			}
		}
		merged.Weights[i] = sum / weightTotal
	}
	for i := range merged.Biases {
		sum := 0.0
		for j, state := range states {
			if i < len(state.Biases) {
				sum += weights[j] * state.Biases[i]
			}
		}
		merged.Biases[i] = sum / weightTotal
	}
	merged.Samples = totalSamples(states)
	return merged, nil
}

func totalSamples(states []ModelState) int64 {
	total := int64(0)
	for _, state := range states {
		total += state.Samples
	}
	return total
}

// ConnectivityScore estimates normalized connectivity based on packet loss,
// peer uptime, and successful publish count. A value near 1 means healthier
// connectivity for weighting in federated aggregation.
func ConnectivityScore(packetLoss float64, peerUptimeSeconds float64, successfulPublishes uint64) float64 {
	if packetLoss < 0 {
		packetLoss = 0
	}
	if packetLoss > 1 {
		packetLoss = 1
	}
	if peerUptimeSeconds < 0 {
		peerUptimeSeconds = 0
	}
	if successfulPublishes == 0 {
		return 0.5 * (1 - packetLoss)
	}
	base := 1.0 - packetLoss
	if peerUptimeSeconds > 0 {
		base *= 1.0 - (1.0 / (1.0 + peerUptimeSeconds/600.0))
	}
	base *= 1.0 - (1.0 / (1.0 + float64(successfulPublishes)/100.0))
	if base < 0.01 {
		return 0.01
	}
	if base > 1.0 {
		return 1.0
	}
	return base
}
