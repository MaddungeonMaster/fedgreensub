package fedgreensub

import (
	"encoding/json"
	"errors"
	"math"
	"sync"
)

// TinyNetwork is a small feedforward neural network with one hidden layer.
type TinyNetwork struct {
	mu sync.RWMutex

	inputSize  int
	hiddenSize int
	outputSize int

	learningRate float64

	// Input-to-hidden weights.
	weightsInputHidden []float64

	// Hidden-layer biases.
	hiddenBiases []float64

	// Hidden-to-output weights.
	weightsHiddenOutput []float64

	// Output-layer biases.
	outputBiases []float64

	version uint64
	samples int64
	loss    float64
}

// NewTinyNetwork creates a small neural network with one hidden layer.
func NewTinyNetwork(
	inputSize int,
	hiddenSize int,
	outputSize int,
	learningRate float64,
) (*TinyNetwork, error) {
	if inputSize <= 0 {
		return nil, errors.New("input size must be greater than zero")
	}

	if hiddenSize <= 0 {
		return nil, errors.New("hidden size must be greater than zero")
	}

	if outputSize <= 0 {
		return nil, errors.New("output size must be greater than zero")
	}

	if learningRate <= 0 {
		return nil, errors.New("learning rate must be greater than zero")
	}

	network := &TinyNetwork{
		inputSize:           inputSize,
		hiddenSize:          hiddenSize,
		outputSize:          outputSize,
		learningRate:        learningRate,
		weightsInputHidden:  make([]float64, inputSize*hiddenSize),
		hiddenBiases:        make([]float64, hiddenSize),
		weightsHiddenOutput: make([]float64, hiddenSize*outputSize),
		outputBiases:        make([]float64, outputSize),
	}

	// Use small deterministic initial values.
	for i := range network.weightsInputHidden {
		network.weightsInputHidden[i] =
			float64((i%7)-3) * 0.01
	}

	for i := range network.weightsHiddenOutput {
		network.weightsHiddenOutput[i] =
			float64((i%5)-2) * 0.01
	}

	return network, nil
}

// Forward performs a forward pass through the neural network.
func (n *TinyNetwork) Forward(
	input []float64,
) ([]float64, error) {
	if n == nil {
		return nil, errors.New("network is nil")
	}

	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.forwardLocked(input)
}

func (n *TinyNetwork) forwardLocked(
	input []float64,
) ([]float64, error) {
	if len(input) != n.inputSize {
		return nil, errors.New("invalid input size")
	}

	hidden := make([]float64, n.hiddenSize)

	for h := 0; h < n.hiddenSize; h++ {
		sum := n.hiddenBiases[h]

		for i := 0; i < n.inputSize; i++ {
			index := i*n.hiddenSize + h
			sum += input[i] * n.weightsInputHidden[index]
		}

		hidden[h] = sigmoid(sum)
	}

	output := make([]float64, n.outputSize)

	for o := 0; o < n.outputSize; o++ {
		sum := n.outputBiases[o]

		for h := 0; h < n.hiddenSize; h++ {
			index := h*n.outputSize + o
			sum += hidden[h] *
				n.weightsHiddenOutput[index]
		}

		output[o] = sigmoid(sum)
	}

	return output, nil
}

// Predict performs inference using the current model weights.
func (n *TinyNetwork) Predict(
	input []float64,
) ([]float64, error) {
	return n.Forward(input)
}

// Backward calculates the loss and updates the network weights using
// backpropagation.
func (n *TinyNetwork) Backward(
	input []float64,
	target []float64,
	output []float64,
) (float64, error) {
	if n == nil {
		return 0, errors.New("network is nil")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if len(input) != n.inputSize {
		return 0, errors.New("invalid input size")
	}

	if len(target) != n.outputSize {
		return 0, errors.New("invalid target size")
	}

	if len(output) != n.outputSize {
		return 0, errors.New("invalid output size")
	}

	// Recalculate hidden activations using the current weights.
	hidden := make([]float64, n.hiddenSize)

	for h := 0; h < n.hiddenSize; h++ {
		sum := n.hiddenBiases[h]

		for i := 0; i < n.inputSize; i++ {
			index := i*n.hiddenSize + h
			sum += input[i] * n.weightsInputHidden[index]
		}

		hidden[h] = sigmoid(sum)
	}

	// Calculate output-layer errors and loss.
	outputDelta := make([]float64, n.outputSize)
	loss := 0.0

	for o := 0; o < n.outputSize; o++ {
		errorValue := output[o] - target[o]

		loss += errorValue * errorValue

		outputDelta[o] =
			errorValue * sigmoidDerivative(output[o])
	}

	loss /= float64(n.outputSize)

	// Calculate hidden-layer errors before updating weights.
	hiddenDelta := make([]float64, n.hiddenSize)

	for h := 0; h < n.hiddenSize; h++ {
		sum := 0.0

		for o := 0; o < n.outputSize; o++ {
			index := h*n.outputSize + o
			sum += outputDelta[o] *
				n.weightsHiddenOutput[index]
		}

		hiddenDelta[h] =
			sum * sigmoidDerivative(hidden[h])
	}

	// Update hidden-to-output weights.
	for h := 0; h < n.hiddenSize; h++ {
		for o := 0; o < n.outputSize; o++ {
			index := h*n.outputSize + o

			n.weightsHiddenOutput[index] -=
				n.learningRate *
					outputDelta[o] *
					hidden[h]
		}
	}

	// Update output biases.
	for o := 0; o < n.outputSize; o++ {
		n.outputBiases[o] -=
			n.learningRate * outputDelta[o]
	}

	// Update input-to-hidden weights.
	for i := 0; i < n.inputSize; i++ {
		for h := 0; h < n.hiddenSize; h++ {
			index := i*n.hiddenSize + h

			n.weightsInputHidden[index] -=
				n.learningRate *
					hiddenDelta[h] *
					input[i]
		}
	}

	// Update hidden biases.
	for h := 0; h < n.hiddenSize; h++ {
		n.hiddenBiases[h] -=
			n.learningRate * hiddenDelta[h]
	}

	n.loss = loss
	n.samples++
	n.version++

	return loss, nil
}

// Train trains the network on all samples in the dataset once.
func (n *TinyNetwork) Train(
	dataset TrainingDataset,
) (float64, error) {
	if n == nil {
		return 0, errors.New("network is nil")
	}

	if len(dataset.Samples) == 0 {
		return 0, errors.New("training dataset is empty")
	}

	totalLoss := 0.0

	for _, sample := range dataset.Samples {
		output, err := n.Forward(sample.Features)
		if err != nil {
			return 0, err
		}

		loss, err := n.Backward(
			sample.Features,
			sample.Targets,
			output,
		)
		if err != nil {
			return 0, err
		}

		totalLoss += loss
	}

	return totalLoss / float64(len(dataset.Samples)), nil
}

// Weights returns a serializable snapshot of the model state.
func (n *TinyNetwork) Weights() ModelState {
	if n == nil {
		return ModelState{}
	}

	n.mu.RLock()
	defer n.mu.RUnlock()

	weights := make(
		[]float64,
		0,
		len(n.weightsInputHidden)+
			len(n.weightsHiddenOutput),
	)

	weights = append(
		weights,
		n.weightsInputHidden...,
	)

	weights = append(
		weights,
		n.weightsHiddenOutput...,
	)

	biases := make(
		[]float64,
		0,
		len(n.hiddenBiases)+
			len(n.outputBiases),
	)

	biases = append(
		biases,
		n.hiddenBiases...,
	)

	biases = append(
		biases,
		n.outputBiases...,
	)

	return ModelState{
		Weights: append([]float64(nil), weights...),
		Biases:  append([]float64(nil), biases...),
		Version: n.version,
		Samples: n.samples,
		Loss:    n.loss,
	}
}

// SetWeights restores a model state.
func (n *TinyNetwork) SetWeights(
	state ModelState,
) error {
	if n == nil {
		return errors.New("network is nil")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	expectedWeights :=
		len(n.weightsInputHidden) +
			len(n.weightsHiddenOutput)

	expectedBiases :=
		len(n.hiddenBiases) +
			len(n.outputBiases)

	if len(state.Weights) != expectedWeights {
		return errors.New("invalid weight count")
	}

	if len(state.Biases) != expectedBiases {
		return errors.New("invalid bias count")
	}

	inputHiddenCount :=
		len(n.weightsInputHidden)

	copy(
		n.weightsInputHidden,
		state.Weights[:inputHiddenCount],
	)

	copy(
		n.weightsHiddenOutput,
		state.Weights[inputHiddenCount:],
	)

	hiddenBiasCount :=
		len(n.hiddenBiases)

	copy(
		n.hiddenBiases,
		state.Biases[:hiddenBiasCount],
	)

	copy(
		n.outputBiases,
		state.Biases[hiddenBiasCount:],
	)

	n.version = state.Version
	n.samples = state.Samples
	n.loss = state.Loss

	return nil
}

// MarshalBinary serializes the model state.
func (n *TinyNetwork) MarshalBinary() ([]byte, error) {
	state := n.Weights()

	return json.Marshal(state)
}

// UnmarshalBinary restores a serialized model state.
func (n *TinyNetwork) UnmarshalBinary(
	data []byte,
) error {
	if n == nil {
		return errors.New("network is nil")
	}

	var state ModelState

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	return n.SetWeights(state)
}

func sigmoid(value float64) float64 {
	return 1.0 / (1.0 + math.Exp(-value))
}

func sigmoidDerivative(output float64) float64 {
	return output * (1.0 - output)
}
