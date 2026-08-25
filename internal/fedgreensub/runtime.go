package fedgreensub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// errNilRuntime is returned when a method is called on a nil *Runtime.
	errNilRuntime = errors.New("fedgreensub: nil runtime")
	// errRuntimeAlreadyRunning is returned by Start when the heartbeat loop
	// is already active.
	errRuntimeAlreadyRunning = errors.New("fedgreensub: runtime already running")
)

// RuntimeStatus is a read-only snapshot of the heartbeat loop's most recent
// activity. It is safe to read while the Runtime is running.
type RuntimeStatus struct {
	Running        bool
	LastMetrics    RuntimeMetrics
	LastParameters GossipParameters
	LastReport     ValidationReport
	LastError      error
}

// Runtime drives the metrics -> prediction -> smoothing -> validation ->
// application pipeline on a heartbeat, without blocking the caller and
// without touching the underlying GossipSub runtime directly. It only
// orchestrates the components that already exist in this package
// (Optimizer, Predictor, ParameterSmoother, ParameterManager).
type Runtime struct {
	mu sync.RWMutex

	config    Config
	optimizer *Optimizer
	predictor Predictor
	smoother  *ParameterSmoother
	params    *ParameterManager

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool

	// metricsBusy/predictionBusy guard against overlapping ticks: if a
	// previous cycle is still running when the next tick fires, that tick
	// is skipped rather than queued or run concurrently.
	metricsBusy    int32
	predictionBusy int32

	status RuntimeStatus
}

// NewRuntime wires the heartbeat loop around already-constructed components.
// optimizer supplies metrics collection and caching, predictor turns metrics
// into suggested GossipParameters, an internally-owned ParameterSmoother
// applies EMA smoothing and safety clamping to those predictions, and params
// validates and safely stores the final result.
func NewRuntime(cfg Config, optimizer *Optimizer, predictor Predictor, params *ParameterManager) *Runtime {
	cfg.normalize()
	return &Runtime{
		config:    cfg,
		optimizer: optimizer,
		predictor: predictor,
		smoother:  NewParameterSmoother(cfg),
		params:    params,
	}
}

// Start begins the background heartbeat loop and returns immediately. The
// loop keeps running in its own goroutines until Stop is called or the
// provided context is done.
func (r *Runtime) Start(ctx context.Context) error {
	if r == nil {
		return errNilRuntime
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return errRuntimeAlreadyRunning
	}

	loopCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.running = true
	r.status.Running = true

	r.wg.Add(2)
	go r.metricsLoop(loopCtx)
	go r.predictionLoop(loopCtx)

	return nil
}

// Stop halts the heartbeat loop and blocks until all background goroutines
// have exited. Calling Stop on a Runtime that was never started, or that has
// already been stopped, is safe and returns immediately.
func (r *Runtime) Stop() {
	if r == nil {
		return
	}

	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	cancel := r.cancel
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	r.wg.Wait()

	r.mu.Lock()
	r.running = false
	r.status.Running = false
	r.mu.Unlock()
}

// Status returns a snapshot of the most recent heartbeat activity.
func (r *Runtime) Status() RuntimeStatus {
	if r == nil {
		return RuntimeStatus{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// IsRunning reports whether the heartbeat loop is currently active.
func (r *Runtime) IsRunning() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

func (r *Runtime) metricsLoop(ctx context.Context) {
	defer r.wg.Done()

	interval := r.config.MetricsInterval
	if interval <= 0 {
		interval = DefaultConfig().MetricsInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tickMetrics(ctx)
		}
	}
}

func (r *Runtime) predictionLoop(ctx context.Context) {
	defer r.wg.Done()

	interval := r.config.PredictionInterval
	if interval <= 0 {
		interval = DefaultConfig().PredictionInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tickPrediction()
		}
	}
}

// tickMetrics refreshes the cached metrics snapshot via the Optimizer. If a
// previous metrics refresh is still in flight this tick is skipped.
func (r *Runtime) tickMetrics(ctx context.Context) {
	if r == nil || r.optimizer == nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&r.metricsBusy, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&r.metricsBusy, 0)

	metrics, err := r.optimizer.RefreshMetrics(ctx)

	r.mu.Lock()
	if err != nil {
		r.status.LastError = err
	} else {
		r.status.LastMetrics = metrics
		r.status.LastError = nil
	}
	r.mu.Unlock()
}

// tickPrediction takes the latest cached metrics snapshot, predicts new
// GossipParameters, smooths them via the ParameterSmoother (EMA blending +
// safety clamping, Phase 10), and validates + applies the result through the
// ParameterManager. If a previous prediction cycle is still in flight this
// tick is skipped.
func (r *Runtime) tickPrediction() {
	if r == nil || r.optimizer == nil || r.predictor == nil || r.params == nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&r.predictionBusy, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&r.predictionBusy, 0)

	metrics := r.optimizer.Snapshot()
	predicted := r.predictor.Predict(metrics)

	if r.smoother != nil {
		predicted = r.smoother.Smooth(predicted)
	}

	report := r.params.ApplyParameters(predicted)

	r.mu.Lock()
	r.status.LastParameters = predicted
	r.status.LastReport = report
	r.mu.Unlock()
}
