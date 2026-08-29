package fedgreensub

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestConfigOptionCoverageAndLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := NewConfig(
		WithLogger(logger),
		WithMinMeshDegree(5),
		WithMaxMeshDegree(18),
		WithMinHeartbeatInterval(750*time.Millisecond),
		WithMaxHeartbeatInterval(3*time.Second),
		WithMinGossipFactor(0.2),
		WithMaxGossipFactor(0.7),
	)

	if cfg.Logger == nil {
		t.Fatal("expected logger to be configured")
	}
	if cfg.MinMeshDegree != 5 || cfg.MaxMeshDegree != 18 {
		t.Fatalf("unexpected mesh bounds: min=%d max=%d", cfg.MinMeshDegree, cfg.MaxMeshDegree)
	}
	if cfg.MinHeartbeatInterval != 750*time.Millisecond || cfg.MaxHeartbeatInterval != 3*time.Second {
		t.Fatalf("unexpected heartbeat bounds: min=%v max=%v", cfg.MinHeartbeatInterval, cfg.MaxHeartbeatInterval)
	}
	if cfg.MinGossipFactor != 0.2 || cfg.MaxGossipFactor != 0.7 {
		t.Fatalf("unexpected gossip bounds: min=%.2f max=%.2f", cfg.MinGossipFactor, cfg.MaxGossipFactor)
	}
}

func TestRuntimeLoggerLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := NewConfig(WithLogger(logger))
	params := NewParameterManager(cfg)
	optimizer := NewOptimizer(cfg, &Collector{systemProbe: NewRuntimeSystemProbe()}, NewHeuristicPredictor(cfg), NewEnergyEstimator(cfg.EnergyWeights))
	rt := NewRuntime(cfg, optimizer, NewHeuristicPredictor(cfg), params)

	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("runtime start failed: %v", err)
	}
	if !rt.IsRunning() {
		t.Fatal("expected runtime to be running")
	}

	rt.Stop()
	if rt.IsRunning() {
		t.Fatal("expected runtime to be stopped")
	}
}
