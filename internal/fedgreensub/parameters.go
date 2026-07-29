package fedgreensub

import (
	"fmt"
	"sync"
)

// ParameterManager validates and safely stores GossipSub parameters.
// It does not modify the original libp2p GossipSub runtime.
type ParameterManager struct {
	mu         sync.RWMutex
	config     Config
	parameters GossipParameters
}

// NewParameterManager creates a new ParameterManager using the
// provided FedGreenSub configuration.
func NewParameterManager(cfg Config) *ParameterManager {
	cfg.normalize()

	return &ParameterManager{
		config: cfg,
		parameters: GossipParameters{
			MeshDegree:        cfg.MinMeshDegree,
			DLow:              cfg.MinMeshDegree,
			DHigh:             cfg.MaxMeshDegree,
			GossipFactor:      cfg.MinGossipFactor,
			HeartbeatInterval: cfg.MinHeartbeatInterval,
		},
	}
}

// Validate checks whether the proposed GossipSub parameters satisfy
// the configured safety limits.
func (p *ParameterManager) Validate(
	params GossipParameters,
) ValidationReport {
	if p == nil {
		return ValidationReport{
			Accepted: false,
			Reason:   "parameter manager is nil",
		}
	}

	// Check mesh degree bounds.
	if params.MeshDegree < p.config.MinMeshDegree {
		return ValidationReport{
			Accepted: false,
			Reason: fmt.Sprintf(
				"mesh degree must be at least %d",
				p.config.MinMeshDegree,
			),
		}
	}

	if params.MeshDegree > p.config.MaxMeshDegree {
		return ValidationReport{
			Accepted: false,
			Reason: fmt.Sprintf(
				"mesh degree must not exceed %d",
				p.config.MaxMeshDegree,
			),
		}
	}

	// Check the relationship:
	// DLow <= MeshDegree <= DHigh
	if params.DLow > params.MeshDegree {
		return ValidationReport{
			Accepted: false,
			Reason:   "DLow must not be greater than MeshDegree",
		}
	}

	if params.DHigh < params.MeshDegree {
		return ValidationReport{
			Accepted: false,
			Reason:   "DHigh must not be less than MeshDegree",
		}
	}

	// Check gossip factor bounds.
	if params.GossipFactor < p.config.MinGossipFactor {
		return ValidationReport{
			Accepted: false,
			Reason: fmt.Sprintf(
				"gossip factor must be at least %.2f",
				p.config.MinGossipFactor,
			),
		}
	}

	if params.GossipFactor > p.config.MaxGossipFactor {
		return ValidationReport{
			Accepted: false,
			Reason: fmt.Sprintf(
				"gossip factor must not exceed %.2f",
				p.config.MaxGossipFactor,
			),
		}
	}

	// Check heartbeat interval bounds.
	if params.HeartbeatInterval < p.config.MinHeartbeatInterval {
		return ValidationReport{
			Accepted: false,
			Reason:   "heartbeat interval is below the configured minimum",
		}
	}

	if params.HeartbeatInterval > p.config.MaxHeartbeatInterval {
		return ValidationReport{
			Accepted: false,
			Reason:   "heartbeat interval exceeds the configured maximum",
		}
	}

	return ValidationReport{
		Accepted: true,
		Reason:   "parameters are valid",
	}
}

// ApplyParameters validates the proposed parameters. If they are valid,
// they are safely stored as the current parameter set.
func (p *ParameterManager) ApplyParameters(
	params GossipParameters,
) ValidationReport {
	if p == nil {
		return ValidationReport{
			Accepted: false,
			Reason:   "parameter manager is nil",
		}
	}

	report := p.Validate(params)

	if !report.Accepted {
		return report
	}

	p.mu.Lock()
	p.parameters = params
	p.mu.Unlock()

	return report
}

// CurrentParameters returns the latest valid parameter set.
func (p *ParameterManager) CurrentParameters() GossipParameters {
	if p == nil {
		return GossipParameters{}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.parameters
}
