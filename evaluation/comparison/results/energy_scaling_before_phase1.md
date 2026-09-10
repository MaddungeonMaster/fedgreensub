# Energy-scaling benchmark before Phase 1

These values were recorded from the pre-Phase-1 run with seed 4242, five repetitions, 200 messages, and peer counts 10/25/50/100/200. They are retained as a comparison record; the raw pre-Phase-1 per-run artifacts were not part of the repository.

| peers | GossipSub total | FedGreenSub total | Trust-Aware total | FedGreen/Trust reduction vs baseline |
|---:|---:|---:|---:|---:|
| 10 | 66.331826 | 151.819705 | 151.819705 | -128.8791% |
| 25 | 68.223211 | 153.709499 | 153.709499 | -125.3038% |
| 50 | 68.336211 | 153.819848 | 153.819848 | -125.0904% |
| 100 | 68.851719 | 154.330053 | 154.330053 | -124.1499% |
| 200 | 68.823118 | 154.301452 | 154.301452 | -124.2024% |

The authoritative post-Phase-1 raw and summary artifacts are `energy_scaling_raw.json`, `energy_scaling_raw.csv`, `energy_scaling_summary.json`, and `energy_scaling_summary.csv` in this directory.
