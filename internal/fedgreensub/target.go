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

// SelectBestCandidate returns the valid candidate with the lowest objective,
// anchored to the currently deployed configuration for deterministic
// tie-breaking. The result must not depend on the order candidates are
// enumerated in.
func SelectBestCandidate(candidates []CandidateScore, minimumDelivery float64, weights TargetWeights, current GossipParameters) (CandidateScore, bool) {
	var best CandidateScore
	found := false
	for _, candidate := range candidates {
		if !candidate.Valid {
			continue
		}
		candidate.Objective = ScoreCandidate(candidate, minimumDelivery, weights)
		if !found || candidateLess(candidate, best, current) {
			best, found = candidate, true
		}
	}
	return best, found
}

// objectiveTieTolerance bounds how close two floating-point objective (or
// derived tie-break) values must be before they are treated as equal rather
// than as a real difference.
const objectiveTieTolerance = 1e-9

// candidateLess reports whether a is a strictly better training-target
// choice than b. It never depends on the order candidates were generated
// in. Priority order:
//
//  1. lower objective, outside objectiveTieTolerance;
//  2. on an objective tie, the candidate requiring the smaller change from
//     the currently deployed configuration (see parameterChangeMagnitude);
//  3. on a further tie, the candidate with the lower modeled resource cost;
//  4. on a full tie, a fixed deterministic parameter ordering (mesh degree,
//     then gossip factor, then heartbeat interval) so the outcome is fully
//     reproducible even when every other criterion is exactly equal.
func candidateLess(a, b CandidateScore, current GossipParameters) bool {
	if diff := a.Objective - b.Objective; math.Abs(diff) > objectiveTieTolerance {
		return diff < 0
	}
	if diff := parameterChangeMagnitude(a.Parameters, current) - parameterChangeMagnitude(b.Parameters, current); math.Abs(diff) > objectiveTieTolerance {
		return diff < 0
	}
	if diff := a.EnergyCost - b.EnergyCost; math.Abs(diff) > objectiveTieTolerance {
		return diff < 0
	}
	if a.Parameters.MeshDegree != b.Parameters.MeshDegree {
		return a.Parameters.MeshDegree < b.Parameters.MeshDegree
	}
	if a.Parameters.GossipFactor != b.Parameters.GossipFactor {
		return a.Parameters.GossipFactor < b.Parameters.GossipFactor
	}
	return a.Parameters.HeartbeatInterval < b.Parameters.HeartbeatInterval
}

// parameterChangeMagnitude is a small, unit-free measure of how far a
// candidate is from the currently deployed configuration. It reuses the same
// per-parameter spans as the relative model encoding (see ParametersTarget /
// ParametersFromModelOutput: mesh +-2, gossip factor +-0.05, heartbeat
// +-25%), so "smaller change" means the same thing here as it does to the
// predictor. It exists only to break objective ties deterministically; it
// never influences selection when objectives differ meaningfully.
func parameterChangeMagnitude(candidate, current GossipParameters) float64 {
	const meshSpan = 2.0
	const gossipSpan = .05
	heartbeatSpan := float64(maxDuration(current.HeartbeatInterval/4, time.Nanosecond))

	meshDelta := math.Abs(float64(candidate.MeshDegree-current.MeshDegree)) / meshSpan
	gossipDelta := math.Abs(candidate.GossipFactor-current.GossipFactor) / gossipSpan
	heartbeatDelta := math.Abs(float64(candidate.HeartbeatInterval-current.HeartbeatInterval)) / heartbeatSpan
	return meshDelta + gossipDelta + heartbeatDelta
}

func ParametersTarget(parameters GossipParameters, cfg Config, current ...GossipParameters) []float64 {
	cfg.normalize()
	if len(current) > 0 {
		base := current[0]
		return []float64{
			clamp01(.5 + float64(parameters.MeshDegree-base.MeshDegree)/4),
			clamp01(.5 + float64(parameters.HeartbeatInterval-base.HeartbeatInterval)/(2*float64(maxDuration(base.HeartbeatInterval/4, time.Nanosecond)))),
			clamp01(.5 + (parameters.GossipFactor-base.GossipFactor)/.1),
		}
	}
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
