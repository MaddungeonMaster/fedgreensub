# Multi-peer modeled resource-cost evaluation

This report is a controlled simulation result. Its cost values are normalized
modeled resource-cost units, not physical energy and not Joules.

## Exact cost model

`internal/fedgreensub/energy.go` computes:

```text
cost = 0.35 * normalize(CPU)
     + 0.25 * normalize(inbound_bandwidth + outbound_bandwidth)
     + 0.15 * normalize(memory)
     + 0.15 * normalize(duplicate_rate)
     + 0.10 * normalize(heartbeat_duration)
```

The normalizers use 100% CPU, 100 MiB/s bandwidth, 4096 MiB memory, a
duplicate-rate range of `[0,1]`, and one second heartbeat duration. The
controlled comparison now calls this same estimator for Standard GossipSub,
FedGreenSub, and Trust-Aware FedGreenSub. Memory is not available in the
controlled participant profiles and is therefore zero in that cost term; this
is explicitly not a physical measurement.

For workload-level reporting:

```text
modeled_resource_cost_total = normalized_cost * published_messages
cost_per_delivered_message = total / delivered_messages
```

The multiplication is a declared workload scaling convention, not a Joule
conversion.

## Reproducible configuration

```text
peer counts:       10, 25, 50, 100, 200
messages:          200
message size:      1024 bytes
publish rate:      100 messages/second
duration:          2 seconds (derived from workload)
FL rounds:         3
repetitions:       5
seeds:             4242, 4243, 4244, 4245, 4246
implementations:   gossipsub, fedgreen, trustaware
```

Run it with:

```powershell
go run ./evaluation/comparison --scenario energy-scaling `
  --peer-counts 10,25,50,100,200 --messages 200 `
  --repetitions 5 --seed 4242 --analyze
```

The participant profiles, topology inputs, workload, and seeds are shared
across methods for each peer count. FL-specific rounds execute only for the FL
methods; Standard GossipSub has no FL round mechanism by design.

## Results

The generated summary is in:

- `results/energy_scaling/energy_scaling_raw.csv`
- `results/energy_scaling/energy_scaling_raw.json`
- `results/energy_scaling/energy_scaling_summary.csv`
- `results/energy_scaling/energy_scaling_summary.json`

Mean modeled total cost and reduction versus Standard GossipSub:

| Peers | Standard | FedGreen | FedGreen reduction | Trust-Aware | Trust reduction |
|---:|---:|---:|---:|---:|---:|
| 10 | 66.3318 | 151.8197 | -128.88% | 151.8197 | -128.88% |
| 25 | 68.2232 | 153.7095 | -125.30% | 153.7095 | -125.30% |
| 50 | 68.3362 | 153.8198 | -125.09% | 153.8198 | -125.09% |
| 100 | 68.8517 | 154.3301 | -124.15% | 154.3301 | -124.15% |
| 200 | 68.8231 | 154.3015 | -124.20% | 154.3015 | -124.20% |

Negative reduction means the modeled cost is higher than the baseline. In this
configuration, the learned FL candidate uses a substantially longer heartbeat
interval than the fixed baseline. Trust-aware aggregation has different
participant weights, but it produces the same final candidate in this sweep,
so its modeled network cost is identical to FedGreenSub.

The raw report also contains delivered messages, duplicates, duplicate ratio,
delivery ratio, bytes, CPU, and Go heap-memory observations. CPU is based on
the deterministic controlled profiles; heap memory is a process-runtime
observation and is not treated as physical power.

## Plots

Dependency-free SVG plots suitable for vector inclusion in a paper are in:

- `plots/output/energy_scaling/energy_cost_vs_peers.svg`
- `plots/output/energy_scaling/energy_cost_per_delivered_vs_peers.svg`
- `plots/output/energy_scaling/energy_reduction_vs_peers.svg`

Regenerate them with:

```powershell
go run ./evaluation/comparison/plots/generate_energy_scaling.go
```

## Interpretation

These results do not support an energy-efficiency claim for FedGreenSub under
this controlled workload. They do support a reproducible scaling comparison
and demonstrate that the current cost model is sensitive to the candidate
heartbeat/resource inputs. A paper should describe this as modeled-resource
cost evidence and should not call it measured energy or Joules. Physical energy
measurement and live distributed-network validation remain future work.
