package fedgreensub

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
