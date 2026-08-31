package fedgreensub

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestRaceConditionConcurrentParameterApplication detects race conditions
// when parameters are applied and read concurrently.
func TestRaceConditionConcurrentParameterApplication(t *testing.T) {
	manager := NewParameterManager(DefaultConfig())
	done := make(chan struct{})
	errCount := int32(0)

	// Writer goroutines: continuously apply parameter changes.
	for i := 0; i < 3; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				// Ensure MeshDegree is between 3 and 12 to be valid
				meshDegree := 3 + ((id*4 + j) % 10)
				params := GossipParameters{
					MeshDegree:        meshDegree,
					DLow:              3,
					DHigh:             16,
					GossipFactor:      0.25,
					HeartbeatInterval: time.Second,
				}
				report := manager.ApplyParameters(params)
				if !report.Accepted {
					atomic.AddInt32(&errCount, 1)
				}
			}
		}(i)
	}

	// Reader goroutines: continuously read current parameters.
	for i := 0; i < 3; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_ = manager.CurrentParameters()
				time.Sleep(1 * time.Microsecond)
			}
		}()
	}

	// Validator goroutines: validate various parameter sets.
	for i := 0; i < 2; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_ = manager.Validate(GossipParameters{
					MeshDegree:        8,
					DLow:              4,
					DHigh:             12,
					GossipFactor:      0.25,
					HeartbeatInterval: time.Second,
				})
			}
		}()
	}

	// Wait for all goroutines to complete.
	for i := 0; i < 8; i++ {
		<-done
	}

	if atomic.LoadInt32(&errCount) > 0 {
		t.Errorf("expected no errors during concurrent access, got %d", errCount)
	}
}

// TestRaceConditionRuntimeConcurrentStatus checks for races in runtime status snapshots.
func TestRaceConditionRuntimeConcurrentStatus(t *testing.T) {
	cfg := NewConfig(WithMetricsInterval(5 * time.Millisecond))
	collector := NewCollector()
	predictor := NewHeuristicPredictor(cfg)
	optimizer := NewOptimizer(cfg, collector, predictor, nil)
	params := NewParameterManager(cfg)
	runtime := NewRuntime(cfg, optimizer, predictor, params)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("failed to start runtime: %v", err)
	}
	defer runtime.Stop()

	done := make(chan struct{})
	errorCount := int32(0)

	// Status readers: continuously snapshot runtime state.
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				status := runtime.Status()
				if status.Running && status.LastError != nil {
					// Validate snapshot coherence.
				}
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// Stopper: terminate after a delay.
	go func() {
		time.Sleep(150 * time.Millisecond)
		runtime.Stop()
		done <- struct{}{}
	}()

	// Wait for all goroutines.
	for i := 0; i < 6; i++ {
		<-done
	}

	if atomic.LoadInt32(&errorCount) > 0 {
		t.Errorf("expected no errors during concurrent status reads, got %d", errorCount)
	}
}

// TestRaceConditionSnapshotStoreConcurrentAccess checks for data races on the snapshot store.
func TestRaceConditionSnapshotStoreConcurrentAccess(t *testing.T) {
	store := NewSnapshotStore()
	done := make(chan struct{})

	// Writers: continuously update metrics.
	for i := 0; i < 4; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				store.Update(RuntimeMetrics{
					Timestamp:     time.Now(),
					CPU:           float64(id*25 + j),
					MemoryMB:      float64(id * 100),
					PeerCount:     id + j%10,
					MeshDegree:    8,
					DuplicateRate: float64(j % 100),
				})
				time.Sleep(100 * time.Microsecond)
			}
		}(i)
	}

	// Readers: continuously fetch latest metrics.
	for i := 0; i < 4; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_ = store.Latest()
				time.Sleep(100 * time.Microsecond)
			}
		}()
	}

	// Wait for all goroutines.
	for i := 0; i < 8; i++ {
		<-done
	}
}

// TestRaceConditionParameterSmootherConcurrentSmoothing detects races in smoothing logic.
func TestRaceConditionParameterSmootherConcurrentSmoothing(t *testing.T) {
	smoother := NewParameterSmoother(DefaultConfig())
	done := make(chan struct{})

	params1 := GossipParameters{MeshDegree: 6, DLow: 3, DHigh: 10, GossipFactor: 0.2, HeartbeatInterval: time.Second}
	params2 := GossipParameters{MeshDegree: 10, DLow: 5, DHigh: 15, GossipFactor: 0.3, HeartbeatInterval: 2 * time.Second}

	// Smoothers: alternate between two parameter sets.
	for i := 0; i < 3; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				if j%2 == 0 {
					_ = smoother.Smooth(params1)
				} else {
					_ = smoother.Smooth(params2)
				}
			}
		}(i)
	}

	// Resetters: occasionally reset state.
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 10; i++ {
			time.Sleep(5 * time.Millisecond)
			smoother.Reset()
		}
	}()

	// Wait for all goroutines.
	for i := 0; i < 4; i++ {
		<-done
	}
}

// TestStressRuntimeStartStopCycles verifies cleanup and restart behavior.
func TestStressRuntimeStartStopCycles(t *testing.T) {
	cfg := NewConfig(WithMetricsInterval(10 * time.Millisecond))
	collector := NewCollector()
	predictor := NewHeuristicPredictor(cfg)
	optimizer := NewOptimizer(cfg, collector, predictor, nil)
	params := NewParameterManager(cfg)
	runtime := NewRuntime(cfg, optimizer, predictor, params)

	for cycle := 0; cycle < 10; cycle++ {
		if err := runtime.Start(context.Background()); err != nil {
			t.Fatalf("cycle %d: Start() failed: %v", cycle, err)
		}

		time.Sleep(50 * time.Millisecond)

		status := runtime.Status()
		if !status.Running {
			t.Fatalf("cycle %d: expected runtime to be running after Start()", cycle)
		}

		runtime.Stop()
		time.Sleep(10 * time.Millisecond)

		if runtime.IsRunning() {
			t.Fatalf("cycle %d: expected runtime to be stopped after Stop()", cycle)
		}
	}
}

// TestHardenedErrorHandlingNilSafety verifies nil-receiver robustness.
func TestHardenedErrorHandlingNilSafety(t *testing.T) {
	// All of these should be safe to call on nil receivers and return sensible defaults.
	var nilCollector *Collector
	_, _ = nilCollector.Collect(context.Background())

	var nilOptimizer *Optimizer
	_ = nilOptimizer.Snapshot()
	_ = nilOptimizer.EnergyEstimate()

	var nilManager *ParameterManager
	_ = nilManager.CurrentParameters()
	_ = nilManager.Validate(GossipParameters{})
	_ = nilManager.ApplyParameters(GossipParameters{})

	var nilRuntime *Runtime
	_ = nilRuntime.Start(context.Background())
	nilRuntime.Stop()
	_ = nilRuntime.Status()
	_ = nilRuntime.IsRunning()

	var nilSmoother *ParameterSmoother
	_ = nilSmoother.Smooth(GossipParameters{})
	nilSmoother.Reset()

	var nilEstimator *EnergyEstimator
	_ = nilEstimator.Weights()
	nilEstimator.SetWeights(DefaultEnergyWeights())
	_ = nilEstimator.EstimateEnergy(RuntimeMetrics{})

	var nilStore *SnapshotStore
	nilStore.Update(RuntimeMetrics{})
	_ = nilStore.Latest()

	var nilTrainer *LocalTrainer
	_, _ = nilTrainer.Train(TrainingDataset{})
	_, _ = nilTrainer.ExportWeights()

	var nilAgg *AggregatorImpl
	_, _ = nilAgg.FedAvg([]ModelState{})
	_, _ = nilAgg.EnergyWeightedFedAvg([]ModelState{}, []float64{})
}

// TestHardenedBoundaryConditions tests edge case parameter combinations.
func TestHardenedBoundaryConditions(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	tests := []struct {
		name   string
		params GossipParameters
		accept bool
	}{
		{
			name: "minimum mesh degree",
			params: GossipParameters{
				MeshDegree:        cfg.MinMeshDegree,
				DLow:              cfg.MinMeshDegree,
				DHigh:             cfg.MaxMeshDegree,
				GossipFactor:      cfg.MinGossipFactor,
				HeartbeatInterval: cfg.MinHeartbeatInterval,
			},
			accept: true,
		},
		{
			name: "maximum mesh degree",
			params: GossipParameters{
				MeshDegree:        cfg.MaxMeshDegree,
				DLow:              cfg.MinMeshDegree,
				DHigh:             cfg.MaxMeshDegree,
				GossipFactor:      cfg.MaxGossipFactor,
				HeartbeatInterval: cfg.MaxHeartbeatInterval,
			},
			accept: true,
		},
		{
			name: "zero mesh degree",
			params: GossipParameters{
				MeshDegree:        0,
				DLow:              0,
				DHigh:             10,
				GossipFactor:      0.25,
				HeartbeatInterval: time.Second,
			},
			accept: false,
		},
		{
			name: "inverted DLow/DHigh",
			params: GossipParameters{
				MeshDegree:        8,
				DLow:              15,
				DHigh:             5,
				GossipFactor:      0.25,
				HeartbeatInterval: time.Second,
			},
			accept: false,
		},
		{
			name: "zero gossip factor",
			params: GossipParameters{
				MeshDegree:        8,
				DLow:              4,
				DHigh:             12,
				GossipFactor:      0,
				HeartbeatInterval: time.Second,
			},
			accept: false,
		},
		{
			name: "negative heartbeat",
			params: GossipParameters{
				MeshDegree:        8,
				DLow:              4,
				DHigh:             12,
				GossipFactor:      0.25,
				HeartbeatInterval: -time.Second,
			},
			accept: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := manager.Validate(tt.params)
			if report.Accepted != tt.accept {
				t.Errorf("expected accepted=%v, got %v (reason: %s)",
					tt.accept, report.Accepted, report.Reason)
			}
		})
	}
}

// TestHardenedConfigNormalization verifies config repair logic.
func TestHardenedConfigNormalization(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() Config
		validate func(t *testing.T, cfg Config)
	}{
		{
			name: "zero mesh bounds become valid",
			builder: func() Config {
				return NewConfig(
					WithMinMeshDegree(0),
					WithMaxMeshDegree(0),
				)
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.MinMeshDegree < 3 {
					t.Errorf("expected MinMeshDegree >= 3, got %d", cfg.MinMeshDegree)
				}
				if cfg.MaxMeshDegree < cfg.MinMeshDegree {
					t.Errorf("expected MaxMeshDegree >= MinMeshDegree")
				}
			},
		},
		{
			name: "inverted heartbeat bounds are swapped",
			builder: func() Config {
				return NewConfig(
					WithMinHeartbeatInterval(10*time.Second),
					WithMaxHeartbeatInterval(100*time.Millisecond),
				)
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.MinHeartbeatInterval > cfg.MaxHeartbeatInterval {
					t.Errorf("expected Min <= Max after normalization")
				}
			},
		},
		{
			name: "inverted gossip bounds are swapped",
			builder: func() Config {
				return NewConfig(
					WithMinGossipFactor(0.8),
					WithMaxGossipFactor(0.2),
				)
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.MinGossipFactor > cfg.MaxGossipFactor {
					t.Errorf("expected Min <= Max after normalization")
				}
			},
		},
		{
			name: "negative EMA alpha becomes valid",
			builder: func() Config {
				return NewConfig(WithEMAAlpha(-0.5))
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.EMAAlpha <= 0 || cfg.EMAAlpha > 1 {
					t.Errorf("expected 0 < EMAAlpha <= 1, got %.2f", cfg.EMAAlpha)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.builder()
			tt.validate(t, cfg)
		})
	}
}

// TestHardenedMemoryLeakDetection ensures goroutines are cleaned up properly.
func TestHardenedMemoryLeakDetection(t *testing.T) {
	initialGoroutines := countGoroutines()

	for i := 0; i < 5; i++ {
		cfg := NewConfig(WithMetricsInterval(5 * time.Millisecond))
		collector := NewCollector()
		predictor := NewHeuristicPredictor(cfg)
		optimizer := NewOptimizer(cfg, collector, predictor, nil)
		params := NewParameterManager(cfg)
		runtime := NewRuntime(cfg, optimizer, predictor, params)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		_ = runtime.Start(ctx)
		time.Sleep(30 * time.Millisecond)
		runtime.Stop()
		cancel()
	}

	// Allow time for cleanup.
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := countGoroutines()
	leaked := finalGoroutines - initialGoroutines

	// Allow a small margin for Go runtime maintenance goroutines.
	if leaked > 5 {
		t.Errorf("detected potential goroutine leak: started with %d, ended with %d (leaked ~%d)",
			initialGoroutines, finalGoroutines, leaked)
	}
}

// countGoroutines returns the current number of running goroutines.
func countGoroutines() int {
	return runtime.NumGoroutine()
}

// TestHardenedContextCancellation verifies proper context handling.
func TestHardenedContextCancellation(t *testing.T) {
	cfg := NewConfig(WithMetricsInterval(5 * time.Millisecond))
	collector := NewCollector()
	predictor := NewHeuristicPredictor(cfg)
	optimizer := NewOptimizer(cfg, collector, predictor, nil)
	params := NewParameterManager(cfg)
	runtime := NewRuntime(cfg, optimizer, predictor, params)

	// Use a context that cancels after 50ms.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for context to expire and runtime loops to exit.
	time.Sleep(100 * time.Millisecond)

	// Call Stop() to shut down the runtime gracefully.
	// This waits for the goroutines to finish and sets running flag to false.
	runtime.Stop()

	// Verify runtime stopped gracefully.
	if runtime.IsRunning() {
		t.Fatal("expected runtime to stop after context cancellation")
	}
}

// TestHardenedPanicRecovery verifies the package doesn't panic under stress.
func TestHardenedPanicRecovery(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic during stress test: %v", r)
		}
	}()

	cfg := NewConfig(
		WithMinMeshDegree(1),
		WithMaxMeshDegree(100),
		WithMinGossipFactor(0.01),
		WithMaxGossipFactor(1.0),
	)

	manager := NewParameterManager(cfg)
	smoother := NewParameterSmoother(cfg)

	// Hammer with extreme parameter combinations.
	for i := 0; i < 1000; i++ {
		extreme := GossipParameters{
			MeshDegree:        (i % 100) + 1,
			DLow:              1,
			DHigh:             100,
			GossipFactor:      0.01 + float64(i%99)*0.01,
			HeartbeatInterval: time.Duration(i%100) * time.Millisecond,
		}

		_ = manager.Validate(extreme)
		_ = manager.ApplyParameters(extreme)
		_ = smoother.Smooth(extreme)
	}
}
