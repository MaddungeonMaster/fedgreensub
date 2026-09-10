package fedgreensub

import "testing"

func TestLearnedPredictorImportsAggregatedGlobalModel(t *testing.T) {
	cfg := DefaultConfig()
	network, err := NewTinyNetwork(ModelInputSize, 8, ModelOutputSize, cfg.LearningRate)
	if err != nil {
		t.Fatal(err)
	}
	predictor := NewLearnedPredictor(network, cfg)
	initial := predictor.ModelState()
	global := cloneModelState(initial)
	global.Weights[0] += 0.25
	global.Biases[len(global.Biases)-1] += 0.5
	if err := predictor.ImportModelState(global); err != nil {
		t.Fatal(err)
	}
	updated := predictor.ModelState()
	if updated.Weights[0] == initial.Weights[0] || updated.Biases[len(updated.Biases)-1] == initial.Biases[len(initial.Biases)-1] {
		t.Fatal("aggregated global model was not installed in predictor")
	}
}

func TestLearnedPredictorPredictionUsesLatestGlobalModel(t *testing.T) {
	cfg := DefaultConfig()
	network, err := NewTinyNetwork(ModelInputSize, 8, ModelOutputSize, cfg.LearningRate)
	if err != nil {
		t.Fatal(err)
	}
	predictor := NewLearnedPredictor(network, cfg)
	current := GossipParameters{MeshDegree: 8, DLow: 4, DHigh: 12, GossipFactor: .25, HeartbeatInterval: 1e9}
	features := RuntimeMetrics{PeerCount: 10}
	before, _ := predictor.PredictWithOutput(features, current)
	global := predictor.ModelState()
	for i := range global.Biases[len(global.Biases)-ModelOutputSize:] {
		global.Biases[len(global.Biases)-ModelOutputSize+i] += 2
	}
	if err := predictor.ImportModelState(global); err != nil {
		t.Fatal(err)
	}
	after, _ := predictor.PredictWithOutput(features, current)
	changed := false
	for i := range before {
		if before[i] != after[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatalf("prediction did not use imported global model: before=%v after=%v", before, after)
	}
}

func TestDifferentAggregatedModelsReachDifferentPredictors(t *testing.T) {
	cfg := DefaultConfig()
	left, _ := NewTinyNetwork(ModelInputSize, 8, ModelOutputSize, cfg.LearningRate)
	right, _ := NewTinyNetwork(ModelInputSize, 8, ModelOutputSize, cfg.LearningRate)
	leftPredictor := NewLearnedPredictor(left, cfg)
	rightPredictor := NewLearnedPredictor(right, cfg)
	leftState := leftPredictor.ModelState()
	rightState := rightPredictor.ModelState()
	leftState.Weights[0] += .1
	rightState.Weights[0] -= .1
	if err := leftPredictor.ImportModelState(leftState); err != nil {
		t.Fatal(err)
	}
	if err := rightPredictor.ImportModelState(rightState); err != nil {
		t.Fatal(err)
	}
	if leftPredictor.ModelState().Weights[0] == rightPredictor.ModelState().Weights[0] {
		t.Fatal("different aggregated models collapsed to the same predictor state")
	}
}
