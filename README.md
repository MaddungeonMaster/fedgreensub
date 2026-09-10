# golang_project

Small collection of Go programs and a `go-libp2p` module used for experiments and coursework.

## Contents

- `test.go` — small runnable example / entrypoint.
- `FedGreenSub_Phase_Status.txt` — notes / status file.
- `go-libp2p/` — a Go module containing libp2p-related code, examples and tests.

## Prerequisites

- Go 1.18 or newer installed and available on your `PATH`.

## Quick commands

Run the simple example in this folder:

```bash
cd "golang_project"
go run test.go
```

Run tests for the `go-libp2p` module:

```bash
cd "golang_project/go-libp2p"
go test ./...
```

Build the module or example binaries:

```bash
cd "golang_project"
go build -o bin/example ./...
```

## Module notes

If you edit or add files inside `go-libp2p`, keep modules tidy:

```bash
cd golang_project/go-libp2p
go mod tidy
```

If you need to run a specific example in `go-libp2p`, change into its folder and use `go run` or `go test` as appropriate.

## FedGreenSub adaptive extension

The `internal/fedgreensub` package is a small adaptive tuning layer for libp2p-style GossipSub behavior. It is designed to collect runtime signals, estimate energy cost, infer safer parameter updates, smooth unstable predictions, and apply validated configuration changes without directly mutating the libp2p runtime.

### What it provides

- runtime metric collection from process and gossip counters
- energy estimation based on CPU, bandwidth, memory, duplicates, and heartbeat cost
- heuristic and lightweight ML-based parameter prediction
- safety validation and bounded parameter application
- EMA smoothing to prevent parameter chattering
- optional runtime heartbeat orchestration
- local training and federated aggregation helpers for adaptive model updates

### Package layout

Key files in the package include:

- `config.go` — global configuration and option helpers
- `collector.go` — metrics collection and snapshot storage
- `optimizer.go` — metric refresh and caching workflow
- `heuristic.go` — rule-based prediction for mesh/heartbeat/gossip settings
- `smoothing.go` — EMA smoothing and clamp enforcement
- `parameters.go` — validation and application of configured parameters
- `runtime.go` — background heartbeat loop and status snapshots
- `trainer.go` and `model.go` — local model training and state export/import
- `aggregation.go` — federated averaging logic and energy-weighted aggregation
- `energy.go` — cost estimation model

### Importing the package

Because the package is under `internal/`, it can only be imported from code inside the same module tree. In practice, code placed under this repository can import it using:

```go
import "golang_project/internal/fedgreensub"
```

### Basic configuration

The package ships with a `DefaultConfig()` and a functional option pattern:

```go
cfg := fedgreensub.NewConfig(
    fedgreensub.WithAdaptiveMode(true),
    fedgreensub.WithFederatedLearning(true),
    fedgreensub.WithEnergyEstimator(true),
    fedgreensub.WithPredictionInterval(2*time.Second),
    fedgreensub.WithMetricsInterval(1*time.Second),
    fedgreensub.WithEMAAlpha(0.25),
)
```

Available option helpers include:

- `WithFederatedLearning(enabled bool)`
- `WithAdaptiveMode(enabled bool)`
- `WithEnergyEstimator(enabled bool)`
- `WithTrainingInterval(interval time.Duration)`
- `WithAggregationInterval(interval time.Duration)`
- `WithPredictionInterval(interval time.Duration)`
- `WithMetricsInterval(interval time.Duration)`
- `WithLogger(logger *slog.Logger)`
- `WithEMAAlpha(alpha float64)`
- `WithMinMeshDegree`, `WithMaxMeshDegree`
- `WithMinHeartbeatInterval`, `WithMaxHeartbeatInterval`
- `WithMinGossipFactor`, `WithMaxGossipFactor`
- `WithMeshBounds`, `WithHeartbeatBounds`, `WithGossipFactorBounds`
- `WithEnergyWeights(weights fedgreensub.EnergyWeights)`

The config also includes enforced safety bounds:

- mesh degree minimum / maximum
- heartbeat interval minimum / maximum
- gossip factor minimum / maximum
- default EMA smoothing alpha
- default energy weights

If invalid values are supplied, `normalize()` resets them to safe defaults.

### Example: creating the runtime loop

```go
package main

import (
    "context"
    "log/slog"
    "time"

    "golang_project/internal/fedgreensub"
)

func main() {
    cfg := fedgreensub.NewConfig(
        fedgreensub.WithAdaptiveMode(true),
        fedgreensub.WithFederatedLearning(true),
        fedgreensub.WithEnergyEstimator(true),
        fedgreensub.WithLogger(slog.Default()),
    )

    collector := fedgreensub.NewCollector()
    optimizer := fedgreensub.NewOptimizer(cfg, collector, fedgreensub.NewHeuristicPredictor(cfg), nil)
    params := fedgreensub.NewParameterManager(cfg)
    runtime := fedgreensub.NewRuntime(cfg, optimizer, fedgreensub.NewHeuristicPredictor(cfg), params)

    ctx := context.Background()
    if err := runtime.Start(ctx); err != nil {
        panic(err)
    }
    defer runtime.Stop()

    for i := 0; i < 5; i++ {
        time.Sleep(500 * time.Millisecond)
        status := runtime.Status()
        slog.Info("runtime snapshot",
            "running", status.Running,
            "mesh_degree", status.LastParameters.MeshDegree,
            "heartbeat", status.LastParameters.HeartbeatInterval,
            "accepted", status.LastReport.Accepted,
        )
    }
}
```

### Metrics collection

`Collector` gathers system and gossip signals into a normalized `RuntimeMetrics` value:

- CPU utilization
- memory usage
- goroutine count
- bandwidth in/out
- duplicate rate
- peer count and mesh degree
- publish latency and heartbeat timing
- packet loss and uptime metrics

The default system probe uses Go runtime metrics, and you can inject custom probes through `WithSystemProbe` and `WithGossipProbe`.

### Prediction and smoothing

The package includes a rule-based `HeuristicPredictor` that suggests new parameters based on the collected runtime metrics. The values are then passed through `ParameterSmoother` before being validated.

This smoothing step applies an exponential moving average and re-clamps the final values within config bounds. The purpose is to avoid oscillation when the predictor repeatedly flips between two values in successive ticks.

### Parameter validation and application

`ParameterManager.Validate()` checks that the proposed set is safe:

- mesh degree stays within configured bounds
- `DLow <= MeshDegree <= DHigh`
- gossip factor remains within bounds
- heartbeat interval stays within bounds

`ApplyParameters()` stores only accepted values and returns a `ValidationReport` describing the result.

### Federated learning support

The package is designed to support local training and federated aggregation:

- `LocalTrainer` trains a lightweight model on samples
- model state can be exported/imported with `ExportWeights()` and `ImportWeights()`
- `AggregatorImpl` can merge results using `FedAvg()` or `EnergyWeightedFedAvg()`

The current implementation also provides a real in-process round coordinator. Candidate GossipSub configurations are scored using delivery, duplicate, latency, and modeled resource costs; the best valid candidate becomes a supervised target. Nodes train `TinyNetwork` locally, export model updates, and install the aggregated global model. The heuristic predictor is not used as the training target.

Aggregation supports sample-count FedAvg, resource/connectivity-weighted FedAvg, and bounded TrustScore-aware FedAvg. TrustScore is a local behavioral reputation signal, not a complete security mechanism.

### Runtime behavior and safety

The background `Runtime` loops operate on independent intervals for metrics collection and prediction. It includes:

- non-blocking tick skipping when a cycle is still in flight
- safe shutdown via `Start(context.Context)` and `Stop()`
- status snapshots via `Status()` and `IsRunning()`
- nil-safety on exported runtime and manager methods
- structured `slog` logging for runtime events

### Good defaults

A conservative starting configuration is:

```go
cfg := fedgreensub.DefaultConfig()
```

This gives a safe baseline with:

- adaptive mode off by default
- federated learning off by default
- energy estimator off by default
- mesh bounds of 3 to 16
- heartbeat bounds of 500ms to 10s
- gossip bounds of 0.1 to 0.5

### Trust-Aware FedGreenSub

Trust-aware mode is an opt-in layer on top of the existing adaptive extension:

```go
cfg := fedgreensub.NewConfig(
    fedgreensub.WithTrustScore(true),
    fedgreensub.WithTrustEMAAlpha(0.25),
    fedgreensub.WithTrustAggregationWeight(1),
)
optimizer := fedgreensub.NewOptimizer(cfg, collector, predictor, nil)
optimizer.ObservePeer(peerID, fedgreensub.PeerObservation{
    PeerID: peerID, MessageReceived: true, DeliverySuccess: true,
    Connected: true,
})
```

`GossipSub PeerScore != FedGreenSub TrustScore`: PeerScore remains the
protocol's existing reputation mechanism. TrustScore is a local, observable
behavioral signal and is not cryptographic proof of identity or honesty. It
does not change message formats, wire behavior, or GossipSub's internal score.

TrustScore uses normalized reliability, observable forwarding, stability,
normalized PeerScore, link quality, and misbehavior evidence. Configured
weights are normalized, and each component is smoothed with an EMA from a
neutral initial value. Invalid messages and abnormal duplication reduce the
misbehavior component; latency and packet loss affect link quality but do not
alone classify a peer as malicious.

The four neighborhood features are average trust, minimum trust, trust
variance, and trusted-peer ratio. They can be added to `RuntimeMetrics` with
`TrustMetrics` and are inputs to communication-intensity decisions; the ML
outputs remain mesh degree, heartbeat interval, and gossip factor. Low trust
only requests additional redundancy when delivery conditions are also poor.
`RankPeers` is limited to FedGreenSub-controlled ranking and never excludes a
peer from GossipSub.

Trust-aware federated aggregation is exposed as `TrustWeightedFedAvg`. It
weights dataset size by trust and the existing connectivity score, uses a
neutral value when trust is unavailable, validates dimensions and finite
parameters, and supports basic norm clipping through `ValidateModelState`.
`TrustModelVersion` identifies feature version 2 (the original feature set is
version 1); old model metadata is not silently treated as a new feature
vector.

Trust mode remains disabled by default, so existing FedGreenSub behavior and
normal GossipSub interoperability are preserved. The implementation is
incremental and bounded: observations update six EMA values under a mutex,
and neighborhood statistics are calculated on periodic metric refreshes.
Benchmark `BenchmarkTrustObservation` measures the additional observation
overhead. Research comparisons should report delivery ratio, duplicate ratio,
latency, mesh degree, estimated energy per delivered message, CPU/memory,
trust overhead, aggregation overhead, convergence, and rejected updates under
normal, churn, low-bandwidth, unstable-peer, and model-outlier scenarios.

### When to enable each feature

- `WithAdaptiveMode(true)` when you want the runtime to adjust parameters over time
- `WithFederatedLearning(true)` when model sharing is part of the deployment design
- `WithEnergyEstimator(true)` for cost-aware decisions and energy-weighted aggregation
- `WithLogger(...)` to attach a custom structured logger in production

### Example custom config

```go
cfg := fedgreensub.NewConfig(
    fedgreensub.WithAdaptiveMode(true),
    fedgreensub.WithFederatedLearning(true),
    fedgreensub.WithEnergyEstimator(true),
    fedgreensub.WithMinMeshDegree(4),
    fedgreensub.WithMaxMeshDegree(20),
    fedgreensub.WithMinHeartbeatInterval(750*time.Millisecond),
    fedgreensub.WithMaxHeartbeatInterval(5*time.Second),
    fedgreensub.WithMinGossipFactor(0.15),
    fedgreensub.WithMaxGossipFactor(0.6),
    fedgreensub.WithEMAAlpha(0.35),
)
```

### Validation status

- Implemented and tested: candidate-based target generation, target-driven TinyNetwork training, in-process federated rounds, bounded energy/resource aggregation, TrustScore-aware participant weighting, and the local GossipSub runtime update fork.
- Controlled/simulation only: the comparison harness's candidate outcomes and participant behavior model. These are not live network measurements.
- Live integration: validated parameter application to a running forked GossipSub router. The public v0.17.0 API does not expose duplicate, delivery, latency, or byte counters, so those metrics require additional instrumentation.
- Modeled resource cost is not physical energy consumption or Joules.

### Notes

- Runtime updates use the local `go-libp2p-pubsub` fork through a serialized router event-loop API. Construction-only fields remain unchanged.
- The live probe reports only metrics exposed by the dependency and does not fabricate unavailable counters.

## Contributing

- Add small README files inside subfolders that need extra run instructions.
- Keep changes focused and test locally with `go test ./...` before pushing.

## License

No license is included. Add a `LICENSE` file at the repo root to declare terms.
