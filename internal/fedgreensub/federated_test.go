package fedgreensub

import (
	"testing"
)

func TestFedAvgAggregatesWeights(t *testing.T) {
	states := []ModelState{
		{Weights: []float64{1, 2}, Biases: []float64{3}, Samples: 10, Loss: 0.1},
		{Weights: []float64{3, 4}, Biases: []float64{5}, Samples: 30, Loss: 0.2},
	}

	agg := NewAggregator()
	got, err := agg.FedAvg(states)
	if err != nil {
		t.Fatalf("FedAvg() error = %v", err)
	}

	if got.Samples != 40 {
		t.Fatalf("FedAvg() samples = %d, want 40", got.Samples)
	}

	if len(got.Weights) != 2 || len(got.Biases) != 1 {
		t.Fatalf("FedAvg() produced wrong shape: weights=%d biases=%d", len(got.Weights), len(got.Biases))
	}

	if got.Weights[0] != 2.5 || got.Weights[1] != 3.5 {
		t.Fatalf("FedAvg() weights = %v, want [2.5 3.5]", got.Weights)
	}

	if got.Biases[0] != 4.5 {
		t.Fatalf("FedAvg() bias = %v, want [4.5]", got.Biases)
	}
}

func TestEnergyWeightedFedAvgUsesBatteryAndConnectivity(t *testing.T) {
	states := []ModelState{
		{Weights: []float64{1, 1}, Biases: []float64{1}, Samples: 10, PacketLossRate: 0.2, PeerUptimeSeconds: 100, SuccessfulPublishes: 20},
		{Weights: []float64{3, 3}, Biases: []float64{3}, Samples: 20, PacketLossRate: 0.1, PeerUptimeSeconds: 200, SuccessfulPublishes: 40},
	}

	agg := NewAggregator()
	got, err := agg.EnergyWeightedFedAvg(states, []float64{0.8, 1.0})
	if err != nil {
		t.Fatalf("EnergyWeightedFedAvg() error = %v", err)
	}

	if got.Samples != 30 {
		t.Fatalf("EnergyWeightedFedAvg() samples = %d, want 30", got.Samples)
	}

	if len(got.Weights) != 2 || len(got.Biases) != 1 {
		t.Fatalf("EnergyWeightedFedAvg() produced wrong shape: weights=%d biases=%d", len(got.Weights), len(got.Biases))
	}
}

func TestLocalTrainerExportsAndImportsWeights(t *testing.T) {
	trainer := NewLocalTrainer()
	trainer.SetState(ModelState{Weights: []float64{1, 2}, Biases: []float64{3}, Samples: 5})

	state, err := trainer.ExportWeights()
	if err != nil {
		t.Fatalf("ExportWeights() error = %v", err)
	}
	if state.Weights[0] != 1 || state.Weights[1] != 2 || state.Biases[0] != 3 {
		t.Fatalf("ExportWeights() returned unexpected state: %+v", state)
	}

	if err := trainer.ImportWeights(ModelState{Weights: []float64{4, 5}, Biases: []float64{6}}); err != nil {
		t.Fatalf("ImportWeights() error = %v", err)
	}

	state, err = trainer.ExportWeights()
	if err != nil {
		t.Fatalf("ExportWeights() after import error = %v", err)
	}
	if state.Weights[0] != 4 || state.Weights[1] != 5 || state.Biases[0] != 6 {
		t.Fatalf("ImportWeights() did not update state: %+v", state)
	}
}

func TestConnectivityScore(t *testing.T) {
	got := ConnectivityScore(0.1, 600, 50)
	if got <= 0 || got > 1 {
		t.Fatalf("ConnectivityScore() = %f, want value in (0,1]", got)
	}
}
