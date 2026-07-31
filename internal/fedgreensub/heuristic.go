package fedgreensub

import "time"

// HeuristicPredictor produces GossipSub parameter suggestions using
// rule-based decisions derived from runtime metrics.
type HeuristicPredictor struct {
	config Config
}

// NewHeuristicPredictor creates a heuristic predictor using the
// provided FedGreenSub configuration.
func NewHeuristicPredictor(cfg Config) *HeuristicPredictor {
	cfg.normalize()

	return &HeuristicPredictor{
		config: cfg,
	}
}

// Predict analyzes runtime metrics and returns a suggested set of
// GossipSub parameters.
//
// The returned parameters are suggestions only. Phase 4 validation
// should be used before applying them.
func (h *HeuristicPredictor) Predict(
	metrics RuntimeMetrics,
) GossipParameters {
	if h == nil {
		return GossipParameters{}
	}

	// Start with conservative default values.
	meshDegree := 8
	gossipFactor := 0.25
	heartbeatInterval := time.Second

	// High CPU usage: reduce mesh size to lower resource usage.
	if metrics.CPU >= 80 {
		meshDegree = 5
	}

	// Low peer count: increase mesh size when possible to improve
	// network connectivity.
	if metrics.PeerCount <= 5 {
		meshDegree = 10
	}

	// High duplicate-message rate: reduce gossip traffic.
	if metrics.DuplicateRate >= 100 {
		gossipFactor = 0.15
	}

	// High publish latency: use a slower heartbeat interval to reduce
	// runtime pressure.
	if metrics.PublishLatency >= 1000 {
		heartbeatInterval = 2 * time.Second
	}

	// Keep all generated values within the configured limits.
	meshDegree = clampInt(
		meshDegree,
		h.config.MinMeshDegree,
		h.config.MaxMeshDegree,
	)

	gossipFactor = clampFloat(
		gossipFactor,
		h.config.MinGossipFactor,
		h.config.MaxGossipFactor,
	)

	heartbeatInterval = clampDuration(
		heartbeatInterval,
		h.config.MinHeartbeatInterval,
		h.config.MaxHeartbeatInterval,
	)

	return GossipParameters{
		MeshDegree:        meshDegree,
		DLow:              meshDegree / 2,
		DHigh:             meshDegree + 4,
		GossipFactor:      gossipFactor,
		HeartbeatInterval: heartbeatInterval,
	}
}

// clampInt limits an integer to the specified range.
func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}

	if value > maximum {
		return maximum
	}

	return value
}

// clampFloat limits a float64 value to the specified range.
func clampFloat(
	value float64,
	minimum float64,
	maximum float64,
) float64 {
	if value < minimum {
		return minimum
	}

	if value > maximum {
		return maximum
	}

	return value
}

// clampDuration limits a duration to the specified range.
func clampDuration(
	value time.Duration,
	minimum time.Duration,
	maximum time.Duration,
) time.Duration {
	if value < minimum {
		return minimum
	}

	if value > maximum {
		return maximum
	}

	return value
}
