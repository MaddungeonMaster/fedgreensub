package fedgreensub

import "context"

// Trainer owns local model updates and weight export/import for federated
// learning rounds.
type Trainer interface {
	Train(context.Context, TrainingDataset) (TrainingResult, error)
	ExportWeights() (ModelState, error)
	ImportWeights(ModelState) error
}

// TrainingResult summarizes one local training pass.
type TrainingResult struct {
	Loss        float64
	SamplesSeen int64
	EnergyCost  float64
	State       ModelState
}
