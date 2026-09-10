package fedgreensub

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Optimizer coordinates metric ingestion, prediction, validation, and safe
// application of GossipSub parameter changes.
type Optimizer struct {
	config    Config
	collector MetricsCollector
	predictor Predictor
	energy    *EnergyEstimator
	store     *SnapshotStore
	trust     *PeerTrustManager
}

// NewOptimizer wires together the extension layer components.
func NewOptimizer(cfg Config, collector MetricsCollector, predictor Predictor, energy *EnergyEstimator) *Optimizer {
	cfg.normalize()
	if energy == nil {
		energy = NewEnergyEstimator(cfg.EnergyWeights)
	}
	optimizer := &Optimizer{
		config:    cfg,
		collector: collector,
		predictor: predictor,
		energy:    energy,
		store:     NewSnapshotStore(),
	}
	if cfg.EnableTrustScore {
		optimizer.trust = NewTrustManager(cfg.TrustWeights, cfg.TrustEMAAlpha)
	}
	return optimizer
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
	if o.config.EnableTrustScore && o.trust != nil {
		metrics = TrustMetrics(metrics, AggregateTrustFeatures(o.trust.AllScores(), o.config.TrustedPeerThreshold))
	}
	o.store.Update(metrics)
	return metrics, nil
}

func (o *Optimizer) ObservePeer(id peer.ID, observation PeerObservation) {
	if o == nil || !o.config.EnableTrustScore || o.trust == nil {
		return
	}
	o.trust.Observe(id, observation)
}

func (o *Optimizer) TrustManager() TrustManager {
	if o == nil || !o.config.EnableTrustScore {
		return nil
	}
	return o.trust
}
