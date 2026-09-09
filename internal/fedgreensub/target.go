package fedgreensub

import (
	"math"
	"time"
)

// TargetWeights defines the supervised target objective. Delivery violations
// receive a strong penalty so lower overhead cannot trade away reliability.
type TargetWeights struct {
	Duplicate float64
	Latency   float64
	Energy    float64
	Delivery  float64
}

func DefaultTargetWeights() TargetWeights {
	return TargetWeights{Duplicate: 1, Latency: .5, Energy: .5, Delivery: 10}
}

func (w TargetWeights) normalize() TargetWeights {
	if w.Duplicate < 0 || w.Latency < 0 || w.Energy < 0 || w.Delivery < 0 || w.Duplicate+w.Latency+w.Energy+w.Delivery == 0 {
		return DefaultTargetWeights()
	}
	return w
}

// ScoreCandidate computes J = duplicate + latency + resource + delivery
// violation penalty. Costs are expected to be normalized to [0,1].
func ScoreCandidate(candidate CandidateScore, minimumDelivery float64, weights TargetWeights) float64 {
	weights = weights.normalize()
	deliveryPenalty := math.Max(0, minimumDelivery-clamp01(candidate.DeliveryRatio))
	return weights.Duplicate*clamp01(candidate.DuplicateRatio) +
		weights.Latency*clamp01(candidate.LatencyCost) +
		weights.Energy*clamp01(candidate.EnergyCost) +
		weights.Delivery*deliveryPenalty*deliveryPenalty
}

// SelectBestCandidate returns the valid candidate with the lowest objective.
func SelectBestCandidate(candidates []CandidateScore, minimumDelivery float64, weights TargetWeights) (CandidateScore, bool) {
	var best CandidateScore
	found := false
	for _, candidate := range candidates {
		if !candidate.Valid {
			continue
		}
		candidate.Objective = ScoreCandidate(candidate, minimumDelivery, weights)
		if !found || candidate.Objective < best.Objective {
			best, found = candidate, true
		}
	}
	return best, found
}

func ParametersTarget(parameters GossipParameters, cfg Config) []float64 {
	cfg.normalize()
	return []float64{
		float64(parameters.MeshDegree-cfg.MinMeshDegree) / float64(maxInt(cfg.MaxMeshDegree-cfg.MinMeshDegree, 1)),
		float64(parameters.HeartbeatInterval-cfg.MinHeartbeatInterval) / float64(maxDuration(cfg.MaxHeartbeatInterval-cfg.MinHeartbeatInterval, time.Nanosecond)),
		(parameters.GossipFactor - cfg.MinGossipFactor) / maxFloat(cfg.MaxGossipFactor-cfg.MinGossipFactor, 1),
	}
}

func maxInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
func maxDuration(value, minimum time.Duration) time.Duration {
	if value < minimum {
		return minimum
	}
	return value
}
func maxFloat(value, minimum float64) float64 {
	if value < minimum {
		return minimum
	}
	return value
}
