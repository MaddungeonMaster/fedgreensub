package fedgreensub

import (
	"math"
	"sync"
	"time"
)

// ParameterSmoother applies exponential moving average (EMA) smoothing to
// predicted GossipParameters before they are validated and applied. This
// damps oscillation between successive predictions (e.g. a heuristic or ML
// predictor flip-flopping between two mesh degrees every tick) so the
// runtime settles toward a stable value instead of chattering.
//
// Every output, smoothed or not, is re-clamped against the same safety
// bounds used elsewhere in the package (Config.Min/Max fields), so smoothing
// can never itself produce an out-of-range or internally inconsistent
// parameter set.
type ParameterSmoother struct {
	mu sync.Mutex

	config Config

	current GossipParameters
	primed  bool
}

// NewParameterSmoother constructs a smoother using the EMA alpha and safety
// bounds from cfg. A larger EMAAlpha (closer to 1) tracks new predictions
// more closely; a smaller value smooths more aggressively.
func NewParameterSmoother(cfg Config) *ParameterSmoother {
	cfg.normalize()
	return &ParameterSmoother{config: cfg}
}

// Smooth blends the newly predicted parameters with the previously smoothed
// value using EMA, clamps the result to the configured safety bounds, and
// returns it. The very first call after construction or after Reset has no
// history to blend against, so it is only clamped, not smoothed.
func (s *ParameterSmoother) Smooth(predicted GossipParameters) GossipParameters {
	if s == nil {
		return predicted
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.primed {
		s.current = s.clampLocked(predicted)
		s.primed = true
		return s.current
	}

	alpha := s.config.EMAAlpha
	if alpha <= 0 || alpha > 1 {
		alpha = DefaultConfig().EMAAlpha
	}

	blended := GossipParameters{
		MeshDegree:        emaInt(s.current.MeshDegree, predicted.MeshDegree, alpha),
		DLow:              emaInt(s.current.DLow, predicted.DLow, alpha),
		DHigh:             emaInt(s.current.DHigh, predicted.DHigh, alpha),
		GossipFactor:      emaFloat(s.current.GossipFactor, predicted.GossipFactor, alpha),
		HeartbeatInterval: emaDuration(s.current.HeartbeatInterval, predicted.HeartbeatInterval, alpha),
		FanoutTTL:         predicted.FanoutTTL,
		PruneBackoff:      predicted.PruneBackoff,
	}

	s.current = s.clampLocked(blended)
	return s.current
}

// Reset clears smoothing history. The next call to Smooth is treated as the
// first sample (clamped only, not blended against stale history) — useful
// after a long pause or a deliberate reconfiguration.
func (s *ParameterSmoother) Reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.primed = false
	s.current = GossipParameters{}
	s.mu.Unlock()
}

// clampLocked re-applies the package's safety bounds. Callers must hold s.mu.
func (s *ParameterSmoother) clampLocked(p GossipParameters) GossipParameters {
	p.MeshDegree = clampInt(p.MeshDegree, s.config.MinMeshDegree, s.config.MaxMeshDegree)
	p.GossipFactor = clampFloat(p.GossipFactor, s.config.MinGossipFactor, s.config.MaxGossipFactor)
	p.HeartbeatInterval = clampDuration(p.HeartbeatInterval, s.config.MinHeartbeatInterval, s.config.MaxHeartbeatInterval)

	// Preserve the DLow <= MeshDegree <= DHigh invariant that
	// ParameterManager.Validate enforces, in case smoothing pulled the mesh
	// degree past either bound.
	if p.DLow > p.MeshDegree {
		p.DLow = p.MeshDegree
	}
	if p.DHigh < p.MeshDegree {
		p.DHigh = p.MeshDegree
	}

	return p
}

func emaInt(previous, current int, alpha float64) int {
	blended := alpha*float64(current) + (1-alpha)*float64(previous)
	return int(math.Round(blended))
}

func emaFloat(previous, current, alpha float64) float64 {
	return alpha*current + (1-alpha)*previous
}

func emaDuration(previous, current time.Duration, alpha float64) time.Duration {
	blended := alpha*float64(current) + (1-alpha)*float64(previous)
	return time.Duration(math.Round(blended))
}
