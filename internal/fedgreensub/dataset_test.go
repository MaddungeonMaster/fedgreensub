package fedgreensub

import "testing"

func TestDatasetWindowRetainsBoundedRecentSamples(t *testing.T) {
	window := NewDatasetWindow(2)
	for value := 0.0; value < 4; value++ {
		window.Append(TrainingSample{Features: []float64{value}, Targets: []float64{value}, Weight: 1})
	}
	dataset := window.Dataset()
	if len(dataset.Samples) != 2 || dataset.Samples[0].Features[0] != 2 || dataset.Samples[1].Features[0] != 3 {
		t.Fatalf("window did not retain recent bounded samples: %+v", dataset.Samples)
	}
}
