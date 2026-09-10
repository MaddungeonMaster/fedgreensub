package fedgreensub

import (
	"context"
	"math"
	"testing"
)

func TestAggregationRejectsMalformedUpdates(t *testing.T) {
	valid := ModelState{Weights: []float64{1}, Biases: []float64{1}, Samples: 1}
	cases := []ModelState{
		{Weights: []float64{math.NaN()}, Biases: []float64{1}, Samples: 1},
		{Weights: []float64{1, 2}, Biases: []float64{1}, Samples: 1},
		{Weights: []float64{1}, Biases: []float64{1}, Samples: -1},
	}
	for _, bad := range cases {
		if _, err := NewAggregator().FedAvg([]ModelState{valid, bad}); err == nil {
			t.Fatalf("FedAvg accepted malformed update: %+v", bad)
		}
	}
	if _, err := NewAggregator().EnergyWeightedFedAvg([]ModelState{valid}, []float64{math.Inf(1)}); err == nil {
		t.Fatal("energy aggregation accepted infinite resource score")
	}
	if _, err := NewAggregator().FedAvg([]ModelState{{Weights: []float64{1}, Biases: []float64{1}, Samples: 0}}); err == nil {
		t.Fatal("FedAvg accepted zero total weight")
	}
}

func TestEnergyAndTrustWeightsAffectInfluence(t *testing.T) {
	states := []ModelState{{Weights: []float64{0}, Biases: []float64{0}, Samples: 10}, {Weights: []float64{10}, Biases: []float64{10}, Samples: 10}}
	energy, err := NewAggregator().EnergyWeightedFedAvg(states, []float64{1, .1})
	if err != nil || energy.Weights[0] >= 5 {
		t.Fatalf("resource metadata did not reduce influence: state=%+v err=%v", energy, err)
	}
	trust, err := NewAggregator().TrustWeightedFedAvg(states, []float64{1, 0}, 1)
	if err != nil || trust.Weights[0] >= 5 {
		t.Fatalf("trust did not reduce unreliable influence: state=%+v err=%v", trust, err)
	}
	if _, err := NewAggregator().TrustWeightedFedAvg(states, []float64{.5, math.NaN()}, 1); err == nil {
		t.Fatal("trust aggregation accepted NaN trust")
	}
}

func TestTrainingSampleWeightChangesUpdate(t *testing.T) {
	low, _ := NewTinyNetwork(1, 2, 1, .5)
	high, _ := NewTinyNetwork(1, 2, 1, .5)
	before := low.Weights()
	_, _ = low.Train(TrainingDataset{Samples: []TrainingSample{{Features: []float64{1}, Targets: []float64{1}, Weight: 1}}})
	_, _ = high.Train(TrainingDataset{Samples: []TrainingSample{{Features: []float64{1}, Targets: []float64{1}, Weight: 10}}})
	lowChange := modelDistance(before, low.Weights())
	highChange := modelDistance(before, high.Weights())
	if highChange <= lowChange {
		t.Fatalf("weighted sample did not have greater update: low=%f high=%f", lowChange, highChange)
	}
	if _, err := low.Train(TrainingDataset{Samples: []TrainingSample{{Features: []float64{1}, Targets: []float64{1}, Weight: -1}}}); err == nil {
		t.Fatal("negative sample weight accepted")
	}
}

type lifecycleEvaluator struct{}

func (lifecycleEvaluator) Evaluate(_ context.Context, _ RuntimeMetrics, candidate GossipParameters) (CandidateScore, error) {
	return CandidateScore{Parameters: candidate, DeliveryRatio: 1, DuplicateRatio: .1, LatencyCost: .1, EnergyCost: .1, Valid: true}, nil
}

func TestRuntimeAddsCandidateSamplesBeforeFLRound(t *testing.T) {
	cfg := DefaultConfig()
	network, err := NewTinyNetwork(ModelInputSize, 4, ModelOutputSize, cfg.LearningRate)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := NewFederatedCoordinator(network.Weights(), cfg.ModelUpdateClipping)
	if err != nil {
		t.Fatal(err)
	}
	local, _ := NewTinyNetwork(ModelInputSize, 4, ModelOutputSize, cfg.LearningRate)
	if err := coordinator.Register(&FederatedParticipant{ID: "p", Trainer: NewNetworkTrainer(local), ResourceWeight: 1}); err != nil {
		t.Fatal(err)
	}
	params := NewParameterManager(cfg)
	predictor := NewLearnedPredictor(network, cfg)
	runtime := NewRuntime(cfg, NewOptimizer(cfg, nil, nil, nil), predictor, params)
	runtime.ConfigureFederatedLearning(coordinator, predictor, FedAvgMethod)
	runtime.ConfigureTrainingData(lifecycleEvaluator{})
	runtime.tickPrediction(context.Background())
	if status := runtime.Status(); status.LastError != nil {
		t.Fatalf("runtime lifecycle failed: %v", status.LastError)
	}
	result, err := coordinator.RunRound(context.Background(), FedAvgMethod)
	if err != nil || result.Contributors != 1 {
		t.Fatalf("runtime did not populate participant window: result=%+v err=%v", result, err)
	}
}
