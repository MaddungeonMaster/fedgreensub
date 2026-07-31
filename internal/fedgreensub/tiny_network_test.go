package fedgreensub

import (
	"math"
	"testing"
)

func TestNewTinyNetwork(t *testing.T) {
	network, err := NewTinyNetwork(3, 4, 2, 0.1)

	if err != nil {
		t.Fatalf("expected network creation to succeed: %v", err)
	}

	if network == nil {
		t.Fatal("expected a non-nil network")
	}
}

func TestNewTinyNetworkRejectsInvalidSizes(t *testing.T) {
	tests := []struct {
		name         string
		inputSize    int
		hiddenSize   int
		outputSize   int
		learningRate float64
	}{
		{
			name:         "zero input size",
			inputSize:    0,
			hiddenSize:   4,
			outputSize:   2,
			learningRate: 0.1,
		},
		{
			name:         "zero hidden size",
			inputSize:    3,
			hiddenSize:   0,
			outputSize:   2,
			learningRate: 0.1,
		},
		{
			name:         "zero output size",
			inputSize:    3,
			hiddenSize:   4,
			outputSize:   0,
			learningRate: 0.1,
		},
		{
			name:         "zero learning rate",
			inputSize:    3,
			hiddenSize:   4,
			outputSize:   2,
			learningRate: 0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			network, err := NewTinyNetwork(
				testCase.inputSize,
				testCase.hiddenSize,
				testCase.outputSize,
				testCase.learningRate,
			)

			if err == nil {
				t.Fatal("expected an error")
			}

			if network != nil {
				t.Fatal("expected a nil network")
			}
		})
	}
}

func TestTinyNetworkForward(t *testing.T) {
	network, err := NewTinyNetwork(3, 4, 2, 0.1)

	if err != nil {
		t.Fatalf("could not create network: %v", err)
	}

	output, err := network.Forward(
		[]float64{0.2, 0.5, 0.8},
	)

	if err != nil {
		t.Fatalf("forward pass failed: %v", err)
	}

	if len(output) != 2 {
		t.Fatalf(
			"expected 2 output values, got %d",
			len(output),
		)
	}

	for index, value := range output {
		if value < 0 || value > 1 {
			t.Errorf(
				"output[%d] = %f is outside [0, 1]",
				index,
				value,
			)
		}
	}
}

func TestTinyNetworkForwardRejectsWrongInputSize(t *testing.T) {
	network, err := NewTinyNetwork(3, 4, 2, 0.1)

	if err != nil {
		t.Fatalf("could not create network: %v", err)
	}

	_, err = network.Forward(
		[]float64{0.2, 0.5},
	)

	if err == nil {
		t.Fatal("expected an invalid input size error")
	}
}

func TestTinyNetworkPredictMatchesForward(t *testing.T) {
	network, err := NewTinyNetwork(3, 4, 2, 0.1)

	if err != nil {
		t.Fatalf("could not create network: %v", err)
	}

	input := []float64{0.2, 0.5, 0.8}

	forwardOutput, err := network.Forward(input)

	if err != nil {
		t.Fatalf("forward pass failed: %v", err)
	}

	prediction, err := network.Predict(input)

	if err != nil {
		t.Fatalf("prediction failed: %v", err)
	}

	if len(prediction) != len(forwardOutput) {
		t.Fatal("prediction and forward output lengths differ")
	}

	for index := range prediction {
		if prediction[index] != forwardOutput[index] {
			t.Errorf(
				"prediction[%d] = %f, expected %f",
				index,
				prediction[index],
				forwardOutput[index],
			)
		}
	}
}

func TestTinyNetworkBackwardUpdatesWeights(t *testing.T) {
	network, err := NewTinyNetwork(2, 3, 1, 0.5)

	if err != nil {
		t.Fatalf("could not create network: %v", err)
	}

	input := []float64{0.4, 0.8}
	target := []float64{1.0}

	before := network.Weights()

	output, err := network.Forward(input)

	if err != nil {
		t.Fatalf("forward pass failed: %v", err)
	}

	loss, err := network.Backward(
		input,
		target,
		output,
	)

	if err != nil {
		t.Fatalf("backward pass failed: %v", err)
	}

	if loss < 0 {
		t.Errorf("loss must not be negative, got %f", loss)
	}

	after := network.Weights()

	if equalFloatSlices(
		before.Weights,
		after.Weights,
	) {
		t.Error("expected weights to change after Backward")
	}

	if after.Samples != before.Samples+1 {
		t.Errorf(
			"expected sample count %d, got %d",
			before.Samples+1,
			after.Samples,
		)
	}
}

func TestTinyNetworkTrainingReducesLoss(t *testing.T) {
	network, err := NewTinyNetwork(2, 4, 1, 0.5)

	if err != nil {
		t.Fatalf("could not create network: %v", err)
	}

	dataset := TrainingDataset{
		Samples: []TrainingSample{
			{
				Features: []float64{0, 0},
				Targets:  []float64{0},
			},
			{
				Features: []float64{0, 1},
				Targets:  []float64{1},
			},
			{
				Features: []float64{1, 0},
				Targets:  []float64{1},
			},
			{
				Features: []float64{1, 1},
				Targets:  []float64{1},
			},
		},
	}

	var firstLoss float64
	var lastLoss float64

	for epoch := 0; epoch < 200; epoch++ {
		loss, err := network.Train(dataset)

		if err != nil {
			t.Fatalf("training failed: %v", err)
		}

		if epoch == 0 {
			firstLoss = loss
		}

		lastLoss = loss
	}

	if lastLoss >= firstLoss {
		t.Errorf(
			"expected loss to decrease: first=%f, last=%f",
			firstLoss,
			lastLoss,
		)
	}
}

func TestTinyNetworkSetWeights(t *testing.T) {
	network, err := NewTinyNetwork(2, 3, 1, 0.1)

	if err != nil {
		t.Fatalf("could not create network: %v", err)
	}

	state := network.Weights()

	state.Weights[0] = 0.75
	state.Biases[0] = 0.25
	state.Version = 10
	state.Samples = 20
	state.Loss = 0.15

	err = network.SetWeights(state)

	if err != nil {
		t.Fatalf("SetWeights failed: %v", err)
	}

	result := network.Weights()

	if result.Weights[0] != 0.75 {
		t.Errorf(
			"expected first weight 0.75, got %f",
			result.Weights[0],
		)
	}

	if result.Biases[0] != 0.25 {
		t.Errorf(
			"expected first bias 0.25, got %f",
			result.Biases[0],
		)
	}

	if result.Version != 10 {
		t.Errorf(
			"expected version 10, got %d",
			result.Version,
		)
	}
}

func TestTinyNetworkMarshalAndUnmarshal(t *testing.T) {
	original, err := NewTinyNetwork(2, 3, 1, 0.1)

	if err != nil {
		t.Fatalf("could not create original network: %v", err)
	}

	data, err := original.MarshalBinary()

	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	restored, err := NewTinyNetwork(2, 3, 1, 0.1)

	if err != nil {
		t.Fatalf("could not create restored network: %v", err)
	}

	err = restored.UnmarshalBinary(data)

	if err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	originalState := original.Weights()
	restoredState := restored.Weights()

	if !equalFloatSlices(
		originalState.Weights,
		restoredState.Weights,
	) {
		t.Error("restored weights do not match")
	}

	if !equalFloatSlices(
		originalState.Biases,
		restoredState.Biases,
	) {
		t.Error("restored biases do not match")
	}
}

func equalFloatSlices(
	first []float64,
	second []float64,
) bool {
	if len(first) != len(second) {
		return false
	}

	for index := range first {
		if math.Abs(
			first[index]-second[index],
		) > 1e-12 {
			return false
		}
	}

	return true
}
