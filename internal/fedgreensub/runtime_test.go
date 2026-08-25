package fedgreensub

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCollector is a MetricsCollector stub that counts calls and returns
// changing values so tests can detect that collection actually ran.
type fakeCollector struct {
	calls int32
}

func (f *fakeCollector) Collect(context.Context) (RuntimeMetrics, error) {
	n := atomic.AddInt32(&f.calls, 1)
	return RuntimeMetrics{
		Timestamp: time.Now(),
		CPU:       float64(n),
	}, nil
}

// fakePredictor is a Predictor stub that counts calls and always returns
// parameters that satisfy DefaultConfig's bounds.
type fakePredictor struct {
	calls int32
}

func (f *fakePredictor) Predict(RuntimeMetrics) GossipParameters {
	atomic.AddInt32(&f.calls, 1)
	return GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}
}

func newTestRuntime() (*Runtime, *fakeCollector, *fakePredictor) {
	cfg := NewConfig(
		WithMetricsInterval(5*time.Millisecond),
		WithPredictionInterval(5*time.Millisecond),
	)

	collector := &fakeCollector{}
	predictor := &fakePredictor{}
	optimizer := NewOptimizer(cfg, collector, predictor, nil)
	params := NewParameterManager(cfg)

	return NewRuntime(cfg, optimizer, predictor, params), collector, predictor
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if condition() {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for condition")
		case <-time.After(2 * time.Millisecond):
		}
	}
}

func TestRuntimeStartPopulatesStatus(t *testing.T) {
	runtime, collector, predictor := newTestRuntime()

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer runtime.Stop()

	waitFor(t, 500*time.Millisecond, func() bool {
		return atomic.LoadInt32(&collector.calls) > 0 && atomic.LoadInt32(&predictor.calls) > 0
	})

	status := runtime.Status()
	if !status.Running {
		t.Fatal("expected Status().Running to be true")
	}
	if !status.LastReport.Accepted {
		t.Fatalf("expected last parameters to be accepted, got: %s", status.LastReport.Reason)
	}
	if status.LastParameters.MeshDegree != 8 {
		t.Fatalf("expected MeshDegree 8, got %d", status.LastParameters.MeshDegree)
	}
	if status.LastError != nil {
		t.Fatalf("expected no error, got %v", status.LastError)
	}
}

func TestRuntimeStopHaltsUpdates(t *testing.T) {
	runtime, collector, _ := newTestRuntime()

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitFor(t, 500*time.Millisecond, func() bool {
		return atomic.LoadInt32(&collector.calls) > 0
	})

	runtime.Stop()

	afterStop := atomic.LoadInt32(&collector.calls)
	time.Sleep(50 * time.Millisecond)
	afterWait := atomic.LoadInt32(&collector.calls)

	if afterWait != afterStop {
		t.Fatalf("expected no further collection after Stop(): before=%d after=%d", afterStop, afterWait)
	}

	if runtime.Status().Running {
		t.Fatal("expected Status().Running to be false after Stop()")
	}
	if runtime.IsRunning() {
		t.Fatal("expected IsRunning() to be false after Stop()")
	}
}

func TestRuntimeDoubleStartRejected(t *testing.T) {
	runtime, _, _ := newTestRuntime()

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	defer runtime.Stop()

	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("expected second Start() to return an error")
	}
}

func TestRuntimeStopWithoutStartIsSafe(t *testing.T) {
	runtime, _, _ := newTestRuntime()
	runtime.Stop() // must not panic or block
	if runtime.IsRunning() {
		t.Fatal("expected IsRunning() to be false")
	}
}

func TestRuntimeStopIsIdempotent(t *testing.T) {
	runtime, _, _ := newTestRuntime()

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	runtime.Stop()
	runtime.Stop() // second call must not panic or block
}

func TestRuntimeRestartAfterStop(t *testing.T) {
	runtime, collector, _ := newTestRuntime()

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	waitFor(t, 500*time.Millisecond, func() bool {
		return atomic.LoadInt32(&collector.calls) > 0
	})
	runtime.Stop()

	firstRunCalls := atomic.LoadInt32(&collector.calls)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	defer runtime.Stop()

	waitFor(t, 500*time.Millisecond, func() bool {
		return atomic.LoadInt32(&collector.calls) > firstRunCalls
	})
}

func TestRuntimeContextCancellationStopsLoop(t *testing.T) {
	runtime, collector, _ := newTestRuntime()

	ctx, cancel := context.WithCancel(context.Background())
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitFor(t, 500*time.Millisecond, func() bool {
		return atomic.LoadInt32(&collector.calls) > 0
	})

	cancel()
	time.Sleep(50 * time.Millisecond)

	afterCancel := atomic.LoadInt32(&collector.calls)
	time.Sleep(50 * time.Millisecond)
	afterWait := atomic.LoadInt32(&collector.calls)

	if afterWait != afterCancel {
		t.Fatalf("expected no further collection after ctx cancellation: before=%d after=%d", afterCancel, afterWait)
	}
}

func TestRuntimeNilSafety(t *testing.T) {
	var runtime *Runtime

	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("expected nil runtime Start() to return an error")
	}
	runtime.Stop()
	if status := runtime.Status(); status.Running {
		t.Fatal("expected nil runtime Status() to report not running")
	}
	if runtime.IsRunning() {
		t.Fatal("expected nil runtime IsRunning() to be false")
	}
}
