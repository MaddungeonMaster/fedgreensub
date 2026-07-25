package fedgreensub

import "context"

// Optimizer coordinates metric ingestion, prediction, validation, and safe
// application of GossipSub parameter changes.
type Optimizer struct {
	config    Config
	collector MetricsCollector
	predictor Predictor
	energy    *EnergyEstimator
	store     *SnapshotStore
}

// NewOptimizer wires together the extension layer components.
func NewOptimizer(cfg Config, collector MetricsCollector, predictor Predictor, energy *EnergyEstimator) *Optimizer {
	cfg.normalize()
	if energy == nil {
		energy = NewEnergyEstimator(cfg.EnergyWeights)
	}
	return &Optimizer{
		config:    cfg,
		collector: collector,
		predictor: predictor,
		energy:    energy,
		store:     NewSnapshotStore(),
	}
}

// Snapshot returns the latest cached metrics sample.
func (o *Optimizer) Snapshot() RuntimeMetrics {
	if o == nil || o.store == nil {
		return RuntimeMetrics{}
	}
	return o.store.Latest()
}

// EnergyEstimate evaluates the latest metrics snapshot using the configured
// estimator.
func (o *Optimizer) EnergyEstimate() float64 {
	if o == nil || o.energy == nil {
		return EstimateEnergy(RuntimeMetrics{})
	}
	return o.energy.EstimateEnergy(o.Snapshot())
}

// RefreshMetrics pulls the latest metrics from the collector if one is wired.
// Later phases will turn this into the heartbeat-driven runtime loop.
func (o *Optimizer) RefreshMetrics(ctx context.Context) (RuntimeMetrics, error) {
	if o == nil || o.collector == nil {
		return RuntimeMetrics{}, nil
	}
	metrics, err := o.collector.Collect(ctx)
	if err != nil {
		return RuntimeMetrics{}, err
	}
	o.store.Update(metrics)
	return metrics, nil
}
