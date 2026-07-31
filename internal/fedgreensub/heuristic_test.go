package fedgreensub

import (
	"testing"
	"time"
)

func TestHeuristicPredictorNormalMetrics(t *testing.T) {
	predictor := NewHeuristicPredictor(DefaultConfig())

	metrics := RuntimeMetrics{
		CPU:            40,
		PeerCount:      10,
		DuplicateRate:  20,
		PublishLatency: 100,
	}

	result := predictor.Predict(metrics)

	if result.MeshDegree != 8 {
		t.Errorf(
			"expected MeshDegree 8, got %d",
			result.MeshDegree,
		)
	}

	if result.GossipFactor != 0.25 {
		t.Errorf(
			"expected GossipFactor 0.25, got %.2f",
			result.GossipFactor,
		)
	}

	if result.HeartbeatInterval != time.Second {
		t.Errorf(
			"expected HeartbeatInterval 1s, got %v",
			result.HeartbeatInterval,
		)
	}
}

func TestHeuristicPredictorHighCPU(t *testing.T) {
	predictor := NewHeuristicPredictor(DefaultConfig())

	metrics := RuntimeMetrics{
		CPU:            90,
		PeerCount:      10,
		DuplicateRate:  20,
		PublishLatency: 100,
	}

	result := predictor.Predict(metrics)

	if result.MeshDegree != 5 {
		t.Errorf(
			"expected MeshDegree 5 for high CPU, got %d",
			result.MeshDegree,
		)
	}
}

func TestHeuristicPredictorLowPeerCount(t *testing.T) {
	predictor := NewHeuristicPredictor(DefaultConfig())

	metrics := RuntimeMetrics{
		CPU:            40,
		PeerCount:      3,
		DuplicateRate:  20,
		PublishLatency: 100,
	}

	result := predictor.Predict(metrics)

	if result.MeshDegree != 10 {
		t.Errorf(
			"expected MeshDegree 10 for low peer count, got %d",
			result.MeshDegree,
		)
	}
}

func TestHeuristicPredictorHighDuplicateRate(t *testing.T) {
	predictor := NewHeuristicPredictor(DefaultConfig())

	metrics := RuntimeMetrics{
		CPU:            40,
		PeerCount:      10,
		DuplicateRate:  150,
		PublishLatency: 100,
	}

	result := predictor.Predict(metrics)

	if result.GossipFactor != 0.15 {
		t.Errorf(
			"expected GossipFactor 0.15 for high duplicate rate, got %.2f",
			result.GossipFactor,
		)
	}
}

func TestHeuristicPredictorHighPublishLatency(t *testing.T) {
	predictor := NewHeuristicPredictor(DefaultConfig())

	metrics := RuntimeMetrics{
		CPU:            40,
		PeerCount:      10,
		DuplicateRate:  20,
		PublishLatency: 1500,
	}

	result := predictor.Predict(metrics)

	expected := 2 * time.Second

	if result.HeartbeatInterval != expected {
		t.Errorf(
			"expected HeartbeatInterval %v, got %v",
			expected,
			result.HeartbeatInterval,
		)
	}
}

func TestHeuristicPredictionStaysWithinBounds(t *testing.T) {
	cfg := NewConfig(
		WithMeshBounds(6, 9),
		WithGossipFactorBounds(0.2, 0.3),
		WithHeartbeatBounds(
			800*time.Millisecond,
			1500*time.Millisecond,
		),
	)

	predictor := NewHeuristicPredictor(cfg)

	metrics := RuntimeMetrics{
		CPU:            90,
		PeerCount:      10,
		DuplicateRate:  150,
		PublishLatency: 1500,
	}

	result := predictor.Predict(metrics)

	if result.MeshDegree < cfg.MinMeshDegree ||
		result.MeshDegree > cfg.MaxMeshDegree {
		t.Errorf(
			"MeshDegree %d is outside configured bounds",
			result.MeshDegree,
		)
	}

	if result.GossipFactor < cfg.MinGossipFactor ||
		result.GossipFactor > cfg.MaxGossipFactor {
		t.Errorf(
			"GossipFactor %.2f is outside configured bounds",
			result.GossipFactor,
		)
	}

	if result.HeartbeatInterval < cfg.MinHeartbeatInterval ||
		result.HeartbeatInterval > cfg.MaxHeartbeatInterval {
		t.Errorf(
			"HeartbeatInterval %v is outside configured bounds",
			result.HeartbeatInterval,
		)
	}
}
