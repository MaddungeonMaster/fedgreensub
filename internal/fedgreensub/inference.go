package fedgreensub

// Predictor produces GossipSub parameter suggestions from runtime metrics.
// The first concrete implementation will be heuristic; the interface keeps the
// ML replacement path isolated.
type Predictor interface {
	Predict(RuntimeMetrics) GossipParameters
}

// PredictionResult binds a prediction to the metrics that produced it so later
// stages can log or validate the outcome.
type PredictionResult struct {
	Metrics    RuntimeMetrics
	Parameters GossipParameters
}
