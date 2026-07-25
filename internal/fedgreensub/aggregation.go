package fedgreensub

// Aggregator combines model updates from multiple peers.
type Aggregator interface {
	FedAvg([]ModelState) (ModelState, error)
	EnergyWeightedFedAvg([]ModelState, []float64) (ModelState, error)
}

// AggregationResult records the outcome of a federated round.
type AggregationResult struct {
	Round        uint64
	Aggregated   ModelState
	Contributors int
	EnergyAware  bool
}
