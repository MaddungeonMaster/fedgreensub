# Isolated FedGreenSub Comparison Harness

This directory is a research evaluation harness, separate from
`internal/fedgreensub` tests and benchmarks. It compares:

1. `gossipsub` - baseline control condition.
2. `fedgreen` - existing FedGreenSub optimizer and energy model.
3. `trustaware` - existing FedGreenSub plus controlled `PeerObservation`
   injection, trust ranking, and `TrustWeightedFedAvg`.

The current repository does not expose a complete automatic GossipSub event
adapter or portable live per-process network counters. The harness therefore
uses a deterministic evaluation-layer workload model for traffic, delivery,
duplicates, latency, and scenario conditions. Go heap memory is measured with
`runtime.MemStats`; portable per-process CPU and live wire-counter values are
not claimed, and their availability flags remain false.
These results must be described as controlled simulation/instrumentation
results, not hardware power measurements or automatic peer-behavior claims.

Run a small experiment:

```powershell
go run ./evaluation/comparison --scenario normal --peers 20 --messages 1000 --repetitions 5 --seed 42
```

Run with analysis output:

```powershell
go run ./evaluation/comparison --scenario packetloss --peers 20 --messages 1000 --repetitions 5 --analyze
```

Raw JSON, CSV, and repetition statistics are written below
`evaluation/comparison/results/<scenario>/`. Existing benchmark files are not
part of this package and are not modified by the harness.
