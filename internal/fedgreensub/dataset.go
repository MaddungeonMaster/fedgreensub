package fedgreensub

import "sync"

type DatasetWindow struct {
	mu      sync.RWMutex
	limit   int
	samples []TrainingSample
}

func NewDatasetWindow(limit int) *DatasetWindow {
	if limit <= 0 {
		limit = DefaultConfig().DatasetWindowSize
	}
	return &DatasetWindow{limit: limit}
}

func (d *DatasetWindow) Append(sample TrainingSample) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.samples = append(d.samples, TrainingSample{Features: append([]float64(nil), sample.Features...), Targets: append([]float64(nil), sample.Targets...), Weight: sample.Weight})
	if len(d.samples) > d.limit {
		d.samples = append([]TrainingSample(nil), d.samples[len(d.samples)-d.limit:]...)
	}
	d.mu.Unlock()
}

func (d *DatasetWindow) Dataset() TrainingDataset {
	if d == nil {
		return TrainingDataset{}
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := TrainingDataset{Samples: make([]TrainingSample, len(d.samples))}
	for i, sample := range d.samples {
		result.Samples[i] = TrainingSample{Features: append([]float64(nil), sample.Features...), Targets: append([]float64(nil), sample.Targets...), Weight: sample.Weight}
	}
	return result
}
