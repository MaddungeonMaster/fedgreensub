# Isolated FedGreenSub Comparison Harness

This directory is a research evaluation harness, separate from
`internal/fedgreensub` tests and benchmarks. It compares four distinct
controlled modes:

1. `gossipsub` - fixed baseline control condition.
2. `heuristic` - rule-based adaptive baseline.
3. `fedgreen` - candidate-targeted local learning and in-process
   resource-aware federated rounds.
4. `trustaware` - the FedGreenSub path plus `PeerTrustManager` observations
   and bounded TrustWeightedFedAvg.

The comparison harness is Level A controlled evaluation. Its candidate
outcomes and workload effects are deterministic model outputs, while local
training, aggregation, model installation, and TrustScore weighting execute
for real. These results must be described as controlled simulation results,
not live GossipSub measurements or hardware power measurements.

The controlled network model is causal within that scope: each candidate's
mesh degree, gossip factor, and heartbeat interval are passed through the
participant profile model to derive delivery, duplicate traffic, latency, and
modeled resource cost. Subsequent FL rounds generate new profile-specific
candidate samples from the resulting conditions. Participant profiles are
seed-dependent and reproducible; repeated runs with the same seed are not
independent statistical samples.

Live GossipSub validation is a separate Level B concern. The v0.17.0 fork
supports synchronized runtime parameter updates, but its public API does not
expose all delivery, duplicate, latency, or wire-byte counters.

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
