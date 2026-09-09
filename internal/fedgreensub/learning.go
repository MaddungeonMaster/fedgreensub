package fedgreensub

import (
	"context"
	"errors"
	"math"
	"sync"
)

// NetworkTrainer adapts TinyNetwork to the federated Trainer contract.
type NetworkTrainer struct {
	mu      sync.RWMutex
	network *TinyNetwork
	epochs  int
}

func NewNetworkTrainer(network *TinyNetwork, epochs ...int) *NetworkTrainer {
	trainingEpochs := 1
	if len(epochs) > 0 && epochs[0] > 0 {
		trainingEpochs = epochs[0]
	}
	return &NetworkTrainer{network: network, epochs: trainingEpochs}
}

func (t *NetworkTrainer) Train(ctx context.Context, dataset TrainingDataset) (TrainingResult, error) {
	if t == nil || t.network == nil {
		return TrainingResult{}, errors.New("fedgreensub: nil network trainer")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(dataset.Samples) == 0 {
		return TrainingResult{}, errors.New("fedgreensub: training dataset is empty")
	}
	loss := 0.0
	for epoch := 0; epoch < t.epochs; epoch++ {
		var err error
		loss, err = t.network.Train(dataset)
		if err != nil {
			return TrainingResult{}, err
		}
	}
	select {
	case <-ctx.Done():
		return TrainingResult{}, ctx.Err()
	default:
	}
	state := t.network.Weights()
	return TrainingResult{Loss: loss, SamplesSeen: state.Samples, State: state}, nil
}

func (t *NetworkTrainer) ExportWeights() (ModelState, error) {
	if t == nil || t.network == nil {
		return ModelState{}, errors.New("fedgreensub: nil network trainer")
	}
	return t.network.Weights(), nil
}

func (t *NetworkTrainer) ImportWeights(state ModelState) error {
	if t == nil || t.network == nil {
		return errors.New("fedgreensub: nil network trainer")
	}
	return t.network.SetWeights(state)
}

// LearnedPredictor converts TinyNetwork output into bounded GossipSub values.
type LearnedPredictor struct {
	network *TinyNetwork
	config  Config
}

func NewLearnedPredictor(network *TinyNetwork, cfg Config) *LearnedPredictor {
	cfg.normalize()
	return &LearnedPredictor{network: network, config: cfg}
}

func (p *LearnedPredictor) Predict(metrics RuntimeMetrics) GossipParameters {
	if p == nil || p.network == nil {
		return GossipParameters{}
	}
	output, err := p.network.Predict(FeatureVector(metrics, p.config))
	if err != nil {
		return GossipParameters{}
	}
	parameters, err := ParametersFromModelOutput(output, p.config)
	if err != nil {
		return GossipParameters{}
	}
	return parameters
}

func modelStateFinite(state ModelState) bool {
	for _, values := range [][]float64{state.Weights, state.Biases} {
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return false
			}
		}
	}
	return true
}
