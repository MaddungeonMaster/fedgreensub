package fedgreensub

import "sync"

// NeuralNetwork is the minimal pure-Go abstraction used for local learning.
// Later phases can back this with a tiny dense feedforward network without
// changing the surrounding package boundaries.
type NeuralNetwork interface {
	Forward(input []float64) ([]float64, error)
	Backward(input, target, output []float64) (float64, error)
	Predict(input []float64) ([]float64, error)
	Train(dataset TrainingDataset) (float64, error)
	Weights() ModelState
	SetWeights(ModelState) error
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
}

// LocalTrainer trains a peer locally and exposes model weights for federated
// aggregation. It intentionally keeps the surface small and serializable.
type LocalTrainer struct {
	mu    sync.RWMutex
	state ModelState
}

func NewLocalTrainer() *LocalTrainer {
	return &LocalTrainer{state: ModelState{Weights: []float64{}, Biases: []float64{}}}
}

func (t *LocalTrainer) SetState(state ModelState) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.state = cloneModelState(state)
	t.mu.Unlock()
}

func (t *LocalTrainer) Train(dataset TrainingDataset) (TrainingResult, error) {
	if t == nil {
		return TrainingResult{}, nil
	}
	if len(dataset.Samples) == 0 {
		return TrainingResult{Loss: 0, SamplesSeen: 0, State: cloneModelState(t.state)}, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	loss := 0.0
	for _, sample := range dataset.Samples {
		if len(sample.Features) == 0 {
			continue
		}
		loss += float64(len(sample.Features)) * sample.Weight
		for i := range t.state.Weights {
			if len(sample.Features) > i {
				t.state.Weights[i] += sample.Features[i] * sample.Weight
			}
		}
	}
	if len(dataset.Samples) > 0 {
		loss /= float64(len(dataset.Samples))
	}
	if len(t.state.Weights) == 0 {
		t.state.Weights = []float64{0}
	}
	if len(t.state.Biases) == 0 {
		t.state.Biases = []float64{0}
	}
	t.state.Samples += int64(len(dataset.Samples))
	t.state.Loss = loss
	return TrainingResult{Loss: loss, SamplesSeen: t.state.Samples, State: cloneModelState(t.state)}, nil
}

func (t *LocalTrainer) ExportWeights() (ModelState, error) {
	if t == nil {
		return ModelState{}, nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return cloneModelState(t.state), nil
}

func (t *LocalTrainer) ImportWeights(state ModelState) error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	t.state = cloneModelState(state)
	t.mu.Unlock()
	return nil
}

func cloneModelState(in ModelState) ModelState {
	out := in
	if len(in.Weights) > 0 {
		out.Weights = append([]float64(nil), in.Weights...)
	}
	if len(in.Biases) > 0 {
		out.Biases = append([]float64(nil), in.Biases...)
	}
	return out
}
