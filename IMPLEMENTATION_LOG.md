# FedGreenSub Implementation Log

Last updated: 2026-09-09

This log records the implementation work completed during the current
FedGreenSub maintenance pass.

## Completed changes

### Runtime to federated-learning data lifecycle

- Added runtime training-data configuration through `Runtime.ConfigureTrainingData`.
- Connected runtime metrics to `GenerateTrainingSample`.
- Candidate configurations are evaluated before a target is created.
- Generated samples are appended to participant `DatasetWindow` instances.
- Federated rounds train from the current dataset window before aggregation.
- Existing candidate-based target generation remains independent of the heuristic predictor.

Relevant files:

- `internal/fedgreensub/runtime.go`
- `internal/fedgreensub/coordinator.go`
- `internal/fedgreensub/dataset.go`
- `internal/fedgreensub/target_generation.go`

### Heterogeneous controlled participants

- Added deterministic participant profiles in the comparison testbed.
- Profiles vary packet loss, latency, duplicate traffic, bandwidth, CPU, and
  modelled resource availability.
- Participant profiles are reproducible and are explicitly controlled
  simulation values, not live measurements.

Relevant file:

- `evaluation/comparison/implementations/implementation.go`

### Energy/resource-aware aggregation

- `EnergyWeightedFedAvg` now validates resource scores and uses them in the
  aggregation weight together with sample count and connectivity.
- Resource availability is documented as modelled metadata, not physical
  energy measurement.
- Invalid resource metadata is rejected by the coordinator and aggregator.

Relevant files:

- `internal/fedgreensub/aggregation.go`
- `internal/fedgreensub/coordinator.go`

### Weighted local training

- `TrainingSample.Weight` now scales both sample loss and gradient updates.
- Zero preserves legacy dataset behavior as neutral weight `1`.
- Negative, NaN, and infinite weights are rejected.

Relevant file:

- `internal/fedgreensub/tiny_network.go`

### Aggregation validation and trust robustness

- FedAvg, energy-weighted FedAvg, and trust-weighted FedAvg now validate:
  - parameter dimensions
  - feature-version compatibility
  - finite model values and metadata
  - non-negative sample counts
  - finite, non-negative aggregation weights
  - non-zero total aggregation weight
- Trust aggregation uses bounded sample influence, connectivity, and a
  documented trust factor.
- Trust scores are bounded to `[0,1]`.
- New participants receive neutral trust unless `TrustKnown` is explicitly set.
- Trust scores cannot dominate solely through excessive sample counts.

Relevant files:

- `internal/fedgreensub/aggregation.go`
- `internal/fedgreensub/coordinator.go`

### FL/heuristic separation

- FL comparison mode no longer constructs or invokes `HeuristicPredictor` as
  its prediction mechanism.
- Heuristic prediction remains available for the dedicated heuristic mode.
- Removed mode-labelled synthetic delivery and duplicate-rate improvements from
  the comparison harness.

Relevant file:

- `evaluation/comparison/implementations/implementation.go`

### Timing and convergence information

- Federated round results now include local training duration and aggregation
  duration.
- Round results include local/global loss semantics and parameter change.
- Comparison metrics now receive actual training and aggregation timings.

Relevant files:

- `internal/fedgreensub/coordinator.go`
- `evaluation/comparison/implementations/implementation.go`

## Tests added

`internal/fedgreensub/federated_hardening_test.go` covers:

- malformed aggregation updates
- NaN/Inf and zero-weight rejection
- resource and trust influence
- weighted training influence
- negative training-weight rejection
- runtime-generated sample availability to FL participants

## Verification

The following command passes:

```text
go test ./...
```

The race suite was attempted with:

```text
go test -race ./...
```

It could not run in the current environment because CGO is enabled for the
race detector but no `gcc` C compiler is installed. This is an environment
limitation, not a test assertion failure.

## Deliberately deferred

The following remain outside this implementation pass:

- distributed FL networking
- physical energy/Joule measurement
- replacing TinyNetwork with a larger ML framework
- rewriting GossipSub
- blockchain, differential privacy, and unrelated security systems

