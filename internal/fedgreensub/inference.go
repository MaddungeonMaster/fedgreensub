package fedgreensub

import (
	"errors"
	"math"
	"time"
)

var errInvalidModelOutput = errors.New("fedgreensub: invalid model output")

// Predictor produces GossipSub parameter suggestions from runtime metrics.
// The first concrete implementation will be heuristic; the interface keeps the
// ML replacement path isolated.
type Predictor interface {
	Predict(RuntimeMetrics) GossipParameters
}

const (
	FeatureVersion       uint64 = 1
	NormalizationVersion uint64 = 1
	ArchitectureVersion  uint64 = 1
	ModelInputSize              = 16
	ModelOutputSize             = 3
)

// FeatureVector converts the runtime contract into normalized model inputs.
// The limits are deliberately conservative and are part of the model schema.
func FeatureVector(metrics RuntimeMetrics, cfg Config) []float64 {
	cfg.normalize()
	return []float64{
		normalizeModelValue(metrics.CPU, 100),
		normalizeModelValue(metrics.MemoryMB, 4096),
		normalizeModelValue(metrics.BandwidthInBps, 100*1024*1024),
		normalizeModelValue(metrics.BandwidthOutBps, 100*1024*1024),
		normalizeModelValue(metrics.IncomingRate, 10000),
		normalizeModelValue(metrics.OutgoingRate, 10000),
		normalizeModelValue(metrics.DuplicateRate, 1000),
		normalizeModelValue(float64(metrics.MeshDegree), float64(cfg.MaxMeshDegree)),
		normalizeModelValue(float64(metrics.PeerCount), 100),
		normalizeModelValue(metrics.PublishLatency, 5000),
		normalizeModelValue(metrics.HeartbeatDuration.Seconds(), cfg.MaxHeartbeatInterval.Seconds()),
		clamp01(metrics.PacketLossRate),
		normalizeModelValue(metrics.PeerUptimeSeconds, 3600),
		clamp01(metrics.AverageNeighborTrust),
		clamp01(metrics.MinimumNeighborTrust),
		clamp01(metrics.TrustedPeerRatio),
	}
}

func normalizeModelValue(value, maximum float64) float64 {
	if maximum <= 0 || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0
	}
	return clamp01(value / maximum)
}

// ParametersFromModelOutput maps the three sigmoid outputs to safe parameter
// candidates. Final protocol validation remains the responsibility of the
// ParameterManager and live GossipSub adapter.
func ParametersFromModelOutput(output []float64, cfg Config) (GossipParameters, error) {
	cfg.normalize()
	if len(output) != ModelOutputSize {
		return GossipParameters{}, errInvalidModelOutput
	}
	for _, value := range output {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return GossipParameters{}, errInvalidModelOutput
		}
	}
	mesh := clampInt(int(math.Round(clamp01(output[0])*float64(cfg.MaxMeshDegree-cfg.MinMeshDegree)+float64(cfg.MinMeshDegree))), cfg.MinMeshDegree, cfg.MaxMeshDegree)
	dLow := clampInt(mesh/2, cfg.MinMeshDegree, mesh)
	dHigh := clampInt(mesh+4, mesh, cfg.MaxMeshDegree)
	return GossipParameters{
		MeshDegree:        mesh,
		DLow:              dLow,
		DHigh:             dHigh,
		GossipFactor:      clampFloat(clamp01(output[2])*(cfg.MaxGossipFactor-cfg.MinGossipFactor)+cfg.MinGossipFactor, cfg.MinGossipFactor, cfg.MaxGossipFactor),
		HeartbeatInterval: clampDuration(time.Duration(clamp01(output[1])*float64(cfg.MaxHeartbeatInterval-cfg.MinHeartbeatInterval)+float64(cfg.MinHeartbeatInterval)), cfg.MinHeartbeatInterval, cfg.MaxHeartbeatInterval),
	}, nil
}

// PredictionResult binds a prediction to the metrics that produced it so later
// stages can log or validate the outcome.
type PredictionResult struct {
	Metrics    RuntimeMetrics
	Parameters GossipParameters
}
