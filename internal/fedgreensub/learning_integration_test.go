package fedgreensub

import (
	"context"
	"testing"
)

type fixedCandidateEvaluator struct {
	best GossipParameters
}

func (e fixedCandidateEvaluator) Evaluate(context.Context, RuntimeMetrics, GossipParameters) (CandidateScore, error) {
	return CandidateScore{Parameters: e.best, DeliveryRatio: 1, Valid: true}, nil
}

func TestGenerateTrainingSampleUsesCandidateOutcome(t *testing.T) {
	cfg := DefaultConfig()
	best := GossipParameters{MeshDegree: 6, DLow: 3, DHigh: 10, GossipFactor: .15, HeartbeatInterval: 2 * 1e9}
	sample, selected, err := GenerateTrainingSample(context.Background(), RuntimeMetrics{}, cfgInitialParameters(cfg), fixedCandidateEvaluator{best: best}, cfg)
	if err != nil {
		t.Fatalf("GenerateTrainingSample() error = %v", err)
	}
	if selected.Parameters != best {
		t.Fatalf("selected candidate = %+v, want %+v", selected.Parameters, best)
	}
	if sample.Targets[0] <= 0 || sample.Targets[2] <= 0 {
		t.Fatalf("candidate target was not mapped to model outputs: %v", sample.Targets)
	}
}

func TestFederatedCoordinatorRunsAndInstallsGlobalModel(t *testing.T) {
	network, err := NewTinyNetwork(ModelInputSize, 6, ModelOutputSize, .1)
	if err != nil {
		t.Fatal(err)
	}
	initial := network.Weights()
	coordinator, err := NewFederatedCoordinator(initial, 10)
	if err != nil {
		t.Fatal(err)
	}
	dataset := TrainingDataset{Samples: []TrainingSample{{Features: make([]float64, ModelInputSize), Targets: []float64{1, 1, 1}, Weight: 1}}}
	for i := 0; i < 2; i++ {
		local, err := NewTinyNetwork(ModelInputSize, 6, ModelOutputSize, .1)
		if err != nil {
			t.Fatal(err)
		}
		if err := coordinator.Register(&FederatedParticipant{ID: string(rune('a' + i)), Trainer: NewNetworkTrainer(local, 2), Dataset: dataset, ResourceWeight: 1, TrustScore: .5}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := coordinator.RunRound(context.Background(), FedAvgMethod)
	if err != nil {
		t.Fatal(err)
	}
	if result.Contributors != 2 || result.Round != 1 {
		t.Fatalf("unexpected round result: %+v", result)
	}
	if result.ParameterChange <= 0 {
		t.Fatalf("expected local learning to change global model: %+v", result)
	}
	if installed := coordinator.GlobalModel(); installed.Version == initial.Version && installed.Loss == initial.Loss {
		t.Fatal("global model was not installed")
	}
}

func cfgInitialParameters(cfg Config) GossipParameters {
	return GossipParameters{MeshDegree: cfg.MinMeshDegree, DLow: cfg.MinMeshDegree, DHigh: cfg.MaxMeshDegree, GossipFactor: cfg.MinGossipFactor, HeartbeatInterval: cfg.MinHeartbeatInterval}
}
