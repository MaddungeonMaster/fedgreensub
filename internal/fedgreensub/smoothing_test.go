package fedgreensub

import (
	"math"
	"testing"
	"time"
)

func TestParameterSmootherFirstCallOnlyClamps(t *testing.T) {
	smoother := NewParameterSmoother(DefaultConfig())

	predicted := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}

	got := smoother.Smooth(predicted)

	if got != predicted {
		t.Fatalf("first Smooth() call = %+v, want unchanged %+v (within bounds, nothing to clamp)", got, predicted)
	}
}

func TestParameterSmootherDampensSuddenChange(t *testing.T) {
	cfg := DefaultConfig() // EMAAlpha = 0.25
	smoother := NewParameterSmoother(cfg)

	first := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}
	smoother.Smooth(first)

	second := GossipParameters{
		MeshDegree:        16,
		DLow:              4,
		DHigh:             16,
		GossipFactor:      0.5,
		HeartbeatInterval: 2 * time.Second,
	}
	got := smoother.Smooth(second)

	// alpha=0.25: blended = 0.25*new + 0.75*old
	wantMeshDegree := 10 // round(0.25*16 + 0.75*8) = round(10) = 10
	if got.MeshDegree != wantMeshDegree {
		t.Fatalf("MeshDegree = %d, want %d (should be damped, not jump straight to %d)", got.MeshDegree, wantMeshDegree, second.MeshDegree)
	}

	wantGossipFactor := 0.3125 // 0.25*0.5 + 0.75*0.25
	if math.Abs(got.GossipFactor-wantGossipFactor) > 1e-9 {
		t.Fatalf("GossipFactor = %f, want %f", got.GossipFactor, wantGossipFactor)
	}

	wantHeartbeat := 1250 * time.Millisecond // 0.25*2s + 0.75*1s
	if got.HeartbeatInterval != wantHeartbeat {
		t.Fatalf("HeartbeatInterval = %v, want %v", got.HeartbeatInterval, wantHeartbeat)
	}

	if got.MeshDegree == second.MeshDegree {
		t.Fatal("expected smoothed MeshDegree to differ from the raw new prediction")
	}
}

func TestParameterSmootherConvergesOverRepeatedTicks(t *testing.T) {
	smoother := NewParameterSmoother(DefaultConfig())

	smoother.Smooth(GossipParameters{MeshDegree: 8, GossipFactor: 0.25, HeartbeatInterval: time.Second})

	target := GossipParameters{MeshDegree: 16, DLow: 4, DHigh: 16, GossipFactor: 0.5, HeartbeatInterval: 2 * time.Second}
	var last GossipParameters
	for i := 0; i < 50; i++ {
		last = smoother.Smooth(target)
	}

	// Integer fields settle within 1 of the target rather than exactly on
	// it: once the rounded value is fed back in as the new "previous"
	// sample, repeated rounding can leave a permanent 1-unit quantization
	// gap (e.g. round(0.25*16 + 0.75*15) = round(15.25) = 15, a stable
	// fixed point one below the target). This is expected, not a bug.
	if diff := target.MeshDegree - last.MeshDegree; diff < 0 || diff > 1 {
		t.Fatalf("expected MeshDegree to settle within 1 of %d after repeated ticks, got %d", target.MeshDegree, last.MeshDegree)
	}
	if math.Abs(last.GossipFactor-target.GossipFactor) > 1e-6 {
		t.Fatalf("expected GossipFactor to converge to %f, got %f", target.GossipFactor, last.GossipFactor)
	}
}

func TestParameterSmootherClampsOutOfRangeValues(t *testing.T) {
	cfg := DefaultConfig()
	smoother := NewParameterSmoother(cfg)

	predicted := GossipParameters{
		MeshDegree:        100,              // above MaxMeshDegree
		DLow:              -5,               // below zero
		DHigh:             1000,             // way above mesh degree, fine
		GossipFactor:      5.0,              // above MaxGossipFactor
		HeartbeatInterval: time.Millisecond, // below MinHeartbeatInterval
	}

	got := smoother.Smooth(predicted)

	if got.MeshDegree != cfg.MaxMeshDegree {
		t.Fatalf("MeshDegree = %d, want clamped to %d", got.MeshDegree, cfg.MaxMeshDegree)
	}
	if got.GossipFactor != cfg.MaxGossipFactor {
		t.Fatalf("GossipFactor = %f, want clamped to %f", got.GossipFactor, cfg.MaxGossipFactor)
	}
	if got.HeartbeatInterval != cfg.MinHeartbeatInterval {
		t.Fatalf("HeartbeatInterval = %v, want clamped to %v", got.HeartbeatInterval, cfg.MinHeartbeatInterval)
	}
	if got.DLow > got.MeshDegree {
		t.Fatalf("DLow (%d) must not exceed MeshDegree (%d)", got.DLow, got.MeshDegree)
	}
}

func TestParameterSmootherReset(t *testing.T) {
	smoother := NewParameterSmoother(DefaultConfig())

	smoother.Smooth(GossipParameters{MeshDegree: 8, GossipFactor: 0.25, HeartbeatInterval: time.Second})
	smoother.Smooth(GossipParameters{MeshDegree: 16, DLow: 4, DHigh: 16, GossipFactor: 0.5, HeartbeatInterval: 2 * time.Second})

	smoother.Reset()

	fresh := GossipParameters{MeshDegree: 12, DLow: 6, DHigh: 16, GossipFactor: 0.4, HeartbeatInterval: 3 * time.Second}
	got := smoother.Smooth(fresh)

	if got != fresh {
		t.Fatalf("after Reset(), first Smooth() = %+v, want unchanged %+v", got, fresh)
	}
}

func TestParameterSmootherNilSafety(t *testing.T) {
	var smoother *ParameterSmoother

	predicted := GossipParameters{MeshDegree: 8, GossipFactor: 0.25, HeartbeatInterval: time.Second}
	got := smoother.Smooth(predicted)
	if got != predicted {
		t.Fatalf("nil smoother Smooth() = %+v, want passthrough %+v", got, predicted)
	}

	smoother.Reset() // must not panic
}
