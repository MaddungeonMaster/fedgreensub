# Evaluation Plotting Module

This module is isolated from the Go implementation and benchmark suite. It
reads raw per-run JSON files under `../results/`, with CSV fallback, and writes
IEEE-style PNG/PDF figures under `plots/output/`.

Run from `golang_project`:

```powershell
& "C:\ProgramData\miniconda3\python.exe" evaluation/comparison/plots/plot_results.py
```

Use alternate input/output directories:

```powershell
& "C:\ProgramData\miniconda3\python.exe" evaluation/comparison/plots/plot_results.py `
  --results evaluation/comparison/results `
  --output evaluation/comparison/plots/output
```

The script also creates:

- `evaluation/comparison/results/plot_summary.csv`
- `evaluation/comparison/results/plot_summary.txt`

Raw results are aggregated by mean and standard deviation across repetitions.
For comparable scenario plots, the largest peer count containing all three
implementations is selected. Missing or unavailable measurements are omitted;
no values are fabricated.

Current data coverage:

- Generated: normal duplicate ratio, duplicate traffic, duplicate ratio vs peers,
  estimated energy vs peers, energy reduction, trust overhead, energy/delivery
  trade-off.
- Not generated: delivery vs packet loss because raw result records do not store
  packet-loss levels; trust distribution because no per-peer category samples
  are stored.

Energy labels intentionally use `Estimated Energy` or `Modeled Energy`; they do
not represent direct hardware power measurements.
