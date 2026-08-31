package fedgreensub

import (
	"context"
	"testing"
	"time"
)

// BenchmarkCollectorBaseline measures raw metrics collection overhead.
func BenchmarkCollectorBaseline(b *testing.B) {
	collector := NewCollector()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = collector.Collect(ctx)
	}
}

// BenchmarkEnergyEstimatorBaseline measures single energy estimate.
func BenchmarkEnergyEstimatorBaseline(b *testing.B) {
	estimator := NewEnergyEstimator(DefaultEnergyWeights())
	metrics := RuntimeMetrics{
		CPU:               75.0,
		MemoryMB:          512.0,
		BandwidthInBps:    1024 * 1024,
		BandwidthOutBps:   1024 * 1024,
		DuplicateRate:     50.0,
		HeartbeatDuration: time.Second,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = estimator.EstimateEnergy(metrics)
	}
}

// BenchmarkHeuristicPredictorBaseline measures heuristic prediction.
func BenchmarkHeuristicPredictorBaseline(b *testing.B) {
	predictor := NewHeuristicPredictor(DefaultConfig())
	metrics := RuntimeMetrics{
		CPU:            50.0,
		DuplicateRate:  25.0,
		PeerCount:      10,
		PublishLatency: 100.0,
		MeshDegree:     8,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = predictor.Predict(metrics)
	}
}

// BenchmarkParameterValidation measures parameter validation cost.
func BenchmarkParameterValidation(b *testing.B) {
	manager := NewParameterManager(DefaultConfig())
	params := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Validate(params)
	}
}

// BenchmarkParameterSmoothing measures EMA smoothing cost.
func BenchmarkParameterSmoothing(b *testing.B) {
	smoother := NewParameterSmoother(DefaultConfig())
	params := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = smoother.Smooth(params)
	}
}

// BenchmarkLocalTrainerTraining measures single local training step.
func BenchmarkLocalTrainerTraining(b *testing.B) {
	trainer := NewLocalTrainer()
	dataset := TrainingDataset{
		Samples: []TrainingSample{
			{Features: []float64{1.0, 2.0}, Targets: []float64{3.0}, Weight: 1.0},
			{Features: []float64{2.0, 3.0}, Targets: []float64{5.0}, Weight: 1.0},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = trainer.Train(dataset)
	}
}

// BenchmarkFedAvgAggregation measures federated averaging of peer states.
func BenchmarkFedAvgAggregation(b *testing.B) {
	agg := NewAggregator()
	models := []ModelState{
		{Weights: []float64{1.0, 2.0, 3.0}, Biases: []float64{0.1}, Version: 1, Samples: 100},
		{Weights: []float64{1.1, 2.1, 3.1}, Biases: []float64{0.11}, Version: 1, Samples: 100},
		{Weights: []float64{0.9, 1.9, 2.9}, Biases: []float64{0.09}, Version: 1, Samples: 100},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = agg.FedAvg(models)
	}
}

// BenchmarkRuntimeMetricsLoop measures background metrics loop overhead.
func BenchmarkRuntimeMetricsLoop(b *testing.B) {
	cfg := NewConfig(WithMetricsInterval(10 * time.Millisecond))
	collector := NewCollector()
	predictor := NewHeuristicPredictor(cfg)
	optimizer := NewOptimizer(cfg, collector, predictor, nil)
	params := NewParameterManager(cfg)
	runtime := NewRuntime(cfg, optimizer, predictor, params)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runtime.Start(ctx); err != nil {
		b.Fatalf("failed to start runtime: %v", err)
	}
	defer runtime.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Let the runtime tick, measure status snapshot cost
		_ = runtime.Status()
	}
}

// BenchmarkFullRuntimeCycle measures end-to-end heartbeat cycle.
func BenchmarkFullRuntimeCycle(b *testing.B) {
	cfg := NewConfig(
		WithMetricsInterval(100*time.Millisecond),
		WithPredictionInterval(100*time.Millisecond),
	)
	collector := NewCollector()
	predictor := NewHeuristicPredictor(cfg)
	optimizer := NewOptimizer(cfg, collector, predictor, nil)
	params := NewParameterManager(cfg)
	runtime := NewRuntime(cfg, optimizer, predictor, params)

	ctx := context.Background()
	if err := runtime.Start(ctx); err != nil {
		b.Fatalf("failed to start runtime: %v", err)
	}
	defer runtime.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		time.Sleep(100 * time.Millisecond)
		_ = runtime.Status()
	}
}

// BenchmarkDefaultConfigCreation measures config initialization cost.
func BenchmarkDefaultConfigCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultConfig()
	}
}

// BenchmarkNewConfigWithOptions measures config building with options.
func BenchmarkNewConfigWithOptions(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewConfig(
			WithAdaptiveMode(true),
			WithFederatedLearning(true),
			WithEMAAlpha(0.25),
			WithMinMeshDegree(4),
			WithMaxMeshDegree(18),
		)
	}
}

// BenchmarkSnapshotStoreUpdateAndRead measures snapshot cache performance.
func BenchmarkSnapshotStoreUpdateAndRead(b *testing.B) {
	store := NewSnapshotStore()
	metrics := RuntimeMetrics{
		Timestamp:      time.Now(),
		CPU:            50.0,
		MemoryMB:       256.0,
		Goroutines:     100,
		PeerCount:      20,
		MeshDegree:     8,
		BandwidthInBps: 1024 * 1024,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Update(metrics)
		_ = store.Latest()
	}
}

// BenchmarkParameterSmootherConvergence measures damping convergence rate.
func BenchmarkParameterSmootherConvergence(b *testing.B) {
	cfg := NewConfig(WithEMAAlpha(0.25))
	smoother := NewParameterSmoother(cfg)

	initial := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}

	target := GossipParameters{
		MeshDegree:        12,
		DLow:              6,
		DHigh:             16,
		GossipFactor:      0.35,
		HeartbeatInterval: 1500 * time.Millisecond,
	}

	b.ResetTimer()
	current := initial
	for i := 0; i < b.N; i++ {
		current = smoother.Smooth(target)
	}
	_ = current
}

// BenchmarkConcurrentParameterManagerReads measures read throughput under contention.
func BenchmarkConcurrentParameterManagerReads(b *testing.B) {
	manager := NewParameterManager(DefaultConfig())
	valid := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}
	_ = manager.ApplyParameters(valid)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = manager.CurrentParameters()
		}
	})
}

// BenchmarkCollectorConcurrentReads measures concurrent metric reads.
func BenchmarkCollectorConcurrentReads(b *testing.B) {
	collector := NewCollector()
	ctx := context.Background()

	// Prime the collector
	_, _ = collector.Collect(ctx)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = collector.Collect(ctx)
		}
	})
}
