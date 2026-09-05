package fedgreensub

import (
	"fmt"
	"math"
)

const TrustModelVersion uint64 = 2

const LegacyModelVersion uint64 = 1

func ValidateModelState(state ModelState, expectedVersion uint64, clipping float64) error {
	if expectedVersion != 0 && state.FeatureVersion != expectedVersion {
		return fmt.Errorf("incompatible model feature version: got %d want %d", state.FeatureVersion, expectedVersion)
	}
	if len(state.Weights) == 0 && len(state.Biases) == 0 {
		return fmt.Errorf("model has no parameters")
	}
	for _, values := range [][]float64{state.Weights, state.Biases} {
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("model contains non-finite parameter")
			}
		}
	}
	if clipping > 0 {
		norm := 0.0
		for _, value := range state.Weights {
			norm += value * value
		}
		for _, value := range state.Biases {
			norm += value * value
		}
		if math.Sqrt(norm) > clipping {
			return fmt.Errorf("model update norm exceeds clipping limit")
		}
	}
	return nil
}
