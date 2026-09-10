# FedGreenSub — Conservative Codebase Cleanup Audit

**Date:** 2026-09-10
**Baseline commit:** `9e29329` ("final benchmarks plus plots"), working tree clean
**Scope:** maintenance/refactoring only. No algorithm, evaluator, FL, guardrail, trust,
target-generation, objective, normalization, seed, configuration, or output-schema change.

## Baseline (recorded before any change)

| Check | Result |
| --- | --- |
| `git status --short` | empty (clean tree, 1203 tracked files) |
| `go build ./...` | exit 0, no output |
| `go test ./...` | all packages `ok` (`golang_project`, `evaluation/comparison`, `.../implementations`, `.../tests`, `internal/fedgreensub`); `.../plots` has no test files |
| `go vet ./...` | 1 finding: `evaluation/comparison/implementations/implementation.go:145:4: self-assignment of parameters` |
| `gofmt -l` | 4 files unformatted (see D4) |

## Method

Declarations were extracted from all 63 non-vendored `.go` files (551 funcs/methods/types),
then each identifier was counted as a whole word across the entire non-vendored source
(including `_test.go`). A count of exactly 1 means "declaration only, zero references".
Every count-1 candidate was then re-verified with an individual `grep -rn`, read in context,
and cross-checked against `README.md`, `evaluation/comparison/plots/README.md`, the shell/
PowerShell scripts, and the CLI entry points. Compiler reachability was confirmed via
`go build ./...` and `go test ./...`. IDE "unused" indicators were not used.

Note: Go's compiler already rejects unused imports and unused local variables, so those
categories are empty by construction and are not listed below.

---

# SAFE TO REMOVE

## A1. `ratioFloat` — `evaluation/comparison/implementations/implementation.go:656`

- **Reason:** unexported helper, zero references anywhere in the module.
- **References found:** 1 (its own declaration). Verified with
  `grep -rn "\bratioFloat\b" --include="*.go"`.
- **Why deletion is safe:** unexported identifiers cannot be referenced outside their
  package, and there are no in-package references. It is not a method, so it cannot satisfy
  an interface. Nothing calls it, so removing it cannot change any computed value.
  The sibling `ratio(n, d uint64)` is a *different* function and **is** used — it stays.

## A2. `minInt` — `evaluation/comparison/implementations/implementation.go:662`

- **Reason:** unexported helper, zero references.
- **References found:** 1 (declaration only).
- **Why deletion is safe:** same argument as A1. Note this is distinct from
  `minInt64` in `internal/fedgreensub/aggregation.go`, which **is** used by
  `TrustWeightedFedAvg` and must not be touched. The sibling `maxInt` in the same file
  **is** used and stays.

## A3. `(*adapter).injectTrust` — `evaluation/comparison/implementations/implementation.go:621`

- **Reason:** unexported method, zero call sites. Superseded prototype.
- **References found:** 1 (declaration only).
- **Why deletion is safe:** `adapter` is an unexported struct, so no external interface can
  require this method, and no in-package interface declares it. It is functionally an earlier
  draft of `(*adapter).recordTrustMetrics` (line 431), which **is** live — called at line 150
  under `if a.config.TrustAware`. Both compute the same four trust features via
  `AggregateTrustFeatures(scores, .7)`. `injectTrust` additionally synthesises peer
  observations and populates `TrustUpdateTime` / `PeerRankingTime` / `TrustAggregationTime`;
  because it is never called, **those three metrics are already always zero today**, so
  removing it changes nothing that is currently produced. This is the only genuine
  duplicate-implementation finding in the audit.

## A4. `(*ExperimentMetrics).finalize` — `evaluation/comparison/metrics.go:66`

- **Reason:** unexported method, zero call sites. Superseded by upstream computation.
- **References found:** 1 (declaration only).
- **Why deletion is safe:** `ExperimentMetrics` lives in `package main`
  (`evaluation/comparison`), so it has no possible external consumer. The ratios it would
  compute are now produced upstream in the implementations package
  (`implementation.go:176-181`, which deliberately preserves the controlled simulator's
  *continuous* ratios) and copied verbatim by `convertMetrics`
  (`evaluation/comparison/runner.go`). Since `finalize` is never invoked, its
  integer-count-derived recomputation never runs. **Deleting it is therefore not only
  behaviour-neutral, it removes a latent hazard:** if it were ever wired up it would
  overwrite the continuous ratios with rounded-count ratios and silently change
  `delivery_ratio` / `duplicate_ratio` in the exported dataset.

## A5. `defaultInferenceParameters` — `internal/fedgreensub/inference.go:100`

- **Reason:** unexported helper, zero references.
- **References found:** 1 (declaration only).
- **Why deletion is safe:** unexported and uncalled. It returns the midpoint of the
  configured bounds — an absolute-parameter fallback superseded by the relative-encoding
  inference path (`DecodedMeshValue` and the relative decode above it), which anchors to the
  *current deployed* parameters rather than to bound midpoints. Removing it cannot affect
  inference because no code path reaches it.

## A6. Self-assignment `parameters = parameters` — `evaluation/comparison/implementations/implementation.go:145`

- **Reason:** provable no-op; sole cause of the repository's only `go vet` finding.
- **Why removal is safe:** assigning a variable to itself is defined to have no effect in
  Go. The explanatory comment `// Keep the last known-good runtime configuration.` is
  **retained** so the intent stays documented. The adjacent `_ = reason` is **retained and
  must not be removed** — `reason` is declared in the `if` initialiser and Go would reject
  the program with "declared and not used" without it. This makes `go vet ./...` clean
  without altering guardrail semantics: the `else` branch still leaves `parameters`
  untouched, exactly as before.

## A7. `evaluation/comparison/plots/__pycache__/plot_results.cpython-313.pyc`

- **Reason:** generated CPython bytecode, committed to git by accident.
- **References found:** 0 in any `.go`, `.py`, `.md`, `.sh`, or `.ps1` file.
- **Why deletion is safe:** `.pyc` files are regenerated automatically by the interpreter
  from `plot_results.py` (which is kept — see K7). It is a build artifact of a specific
  interpreter version (3.13) that is not even the version now used for analysis (3.12).
  Deleting it cannot affect any Go or Python result.

## A8. `evaluation/comparison/plots/output.zip`

- **Reason:** redundant 628 KB binary archive duplicating already-tracked files.
- **References found:** 0 anywhere in the repository.
- **Why deletion is safe:** verified its 14 members are byte-size-identical to the 14
  individually tracked files already present in `evaluation/comparison/plots/output/`
  (all 14 sizes matched exactly, e.g. `01_normal_duplicate_ratio.png` = 133622 bytes in
  both). The archive is a snapshot of a directory that is itself committed, so no content
  is lost. Nothing reads it.

---

# PROBABLY OBSOLETE — NEEDS REVIEW (not touched)

## B1. `AggregationResult` — `internal/fedgreensub/aggregation.go:13`

- **Status:** exported struct, zero references module-wide.
- **Why not removed:** it is *exported* API surface of the FL package. Although `internal/`
  means no out-of-module consumer can exist today, this is a plainly-intended result type
  for aggregation rounds (`Round`, `Aggregated`, `Contributors`, `EnergyAware`) and may be
  the intended return type for future coordinator work. Per the "when uncertain, keep it"
  rule, flagged rather than deleted. **Your call.**

## B2. `evaluation/comparison/results/energy-scaling/` — 626 JSON files, 9.3 MB

- **Status:** accumulated per-run outputs spanning **505 distinct experiment IDs**, i.e.
  many repeated historical runs, all tracked in git.
- **Why not removed:** these are produced by `saveResult` and are the raw inputs behind
  `results/energy_scaling/energy_scaling_summary.csv`, which in turn feeds
  `plots/generate_energy_scaling.go` and `plots/plot_energy_scaling.py`. Only the most
  recent run is actually needed to regenerate the report, so most of the 626 are almost
  certainly stale — but "which run produced the committed summary" cannot be determined
  safely from filenames alone. Pruning to the newest experiment ID would reclaim most of the
  9.3 MB. **Needs your decision on which run is canonical.**
- **Important:** `results/energy-scaling/` (hyphen, per-run JSON) and
  `results/energy_scaling/` (underscore, aggregated report) look like a naming duplication
  but are **not** duplicates — they are two distinct, both-live outputs of the same scenario
  (`main.go:56-57` vs `saveResult`). Neither should be merged or renamed.

## B3. `compare-implementations.sh` — RESOLVED (follow-up task)

> **Correction to this entry.** It originally named *both* `compare-implementations.sh` and
> `compare-implementations.ps1`. That over-generalized: verification showed **every
> fabricated claim was in the `.sh` only**. `compare-implementations.ps1` was already
> measurement-driven — it runs both benchmark suites, parses the real `go test -bench`
> output, prints a measured table, and states "The comparison above is based on actual
> benchmark output from the current workspace." It contains no fabricated numbers and was
> **left completely unmodified**.

- **Status:** `compare-implementations.sh` was ~95% hardcoded `echo` statements presenting
  **fabricated numbers** as results, before running four real benchmarks at the end.
- **Claims removed** (all contradicted the validated terminology and results):
  `Energy Cost: 2.5 Joules/minute` / `1.2 Joules/minute`, `Battery Drain Rate: 15% per hour`
  / `7% per hour`, `Operation Time: ~6.5 hours` / `~14 hours on 100% battery`,
  `52% less CPU consumed`, `70% bandwidth reduction`, `52% longer battery life`,
  `Savings: 50-70% in bandwidth/energy under high load`,
  `Overhead: Minimal (5-15% CPU)`, plus invented per-scenario mesh/heartbeat/gossip/
  bandwidth figures for four fictional scenarios. The validated pipeline reports a
  **modeled, normalized resource-cost proxy** — never Joules — with reductions of
  **0.42%–1.88%**.
- **Resolution:** the `.sh` was rewritten to mirror the `.ps1`'s honest structure: it runs
  the same four real FedGreenSub microbenchmarks it always ran, additionally runs the
  original GossipSub suite (existing targets `BenchmarkOriginalGossipSub{,Concurrent}Publish`)
  so it genuinely compares implementations as its name claims, prints only real
  `go test -bench` output, propagates a non-zero exit code on failure, and carries a header
  noting the resource-cost-proxy terminology and pointing at the validated dataset pipeline.
  **No fabricated value was replaced with a new hardcoded value** — no summary numbers are
  printed at all.

## B4. `go-libp2p/` — 779 files, untracked

- **Status:** listed in `.gitignore`, so **0 files tracked**. It is a nested module with its
  own `go.mod`, therefore excluded from `./...` automatically. `go.mod` has a `replace` for
  `go-libp2p-pubsub` → `./third_party/go-libp2p-pubsub`, but **no** replace for
  `go-libp2p`; the dependency `github.com/libp2p/go-libp2p v0.47.0` resolves from the module
  cache instead. The directory is therefore unreferenced by the build.
- **Why not removed:** it is already invisible to the repository (gitignored), so it costs
  the repo nothing, and it may be a deliberate local reference checkout. Deleting it is a
  local-disk decision, not a repository cleanup. **Left entirely untouched.**

## B5. Stale documentation statements

Not modified — corrections would touch user-facing docs beyond a dead-code audit.

- `README.md:7,9` lists `go-libp2p/` as repository contents and gives `cd golang_project/go-libp2p; go test ./...`
  instructions, but the directory is gitignored and absent from a fresh clone.
- `README.md` "Prerequisites: Go 1.18 or newer" vs `go.mod` `go 1.26`.
- `evaluation/comparison/main.go:20` — the `-scenario` flag help string lists
  `normal, scaling, highload, packetloss, duplicate, churn, trust, fl` but omits
  `energy-scaling`, which **is** a supported case (`runner.go:108`, `main.go:56`).
  Fixing the help text would alter CLI output, which is out of scope for this task.

---

# KEEP — CURRENTLY USED

| Item | Evidence |
| --- | --- |
| `ratio` (implementation.go) | called; distinct from the dead `ratioFloat` |
| `maxInt` (implementation.go) | called; distinct from the dead `minInt` |
| `minInt64` (aggregation.go) | used by `TrustWeightedFedAvg` sample capping |
| `(*adapter).recordTrustMetrics` | called at `implementation.go:150` |
| `(*adapter).optimizerPrediction` | referenced |
| `test.go` (root `package main`) | documented entry point: `README.md:21` → `go run test.go` |
| `original_gossipsub_benchmarks_test.go` | benchmark entry points in the root package |
| `evaluation/comparison/plots/generate_energy_scaling.go` | separate `main` built by `go build ./...`; reads `results/energy_scaling/` |
| `plots/plot_results.py` | documented in `plots/README.md:10,16` |
| `plots/plot_energy_scaling.py` | reads `results/energy_scaling/energy_scaling_summary.csv` |
| `results/{normal,duplicate,packetloss,scaling,trust}/` | raw inputs consumed by `plot_results.py` to produce the tracked `plots/output/` figures |
| `results/energy_scaling/` (underscore) | report written by `writeEnergyScalingReport`; read by both plot generators |
| All `run-benchmarks.*`, `run-phases-14-15.ps1` | invoke real `go test -bench` targets |

## KEEP — documented public API (unused in-module, but intentional)

`internal/fedgreensub/config.go` exposes 29 `With*` functional options; 11 have zero
in-module call sites (`WithAggregationInterval`, `WithEnergyEstimator`, `WithEnergyWeights`,
`WithMinimumTrust`, `WithModelUpdateClipping`, `WithTrainingInterval`,
`WithTrustAggregationWeight`, `WithTrustEMAAlpha`, `WithTrustObservationInterval`,
`WithTrustWeights`, `WithTrustedPeerThreshold`).

**All of them are explicitly documented in `README.md`** — several appear in the README's
worked configuration examples (`WithEnergyEstimator`, `WithTrainingInterval`,
`WithAggregationInterval`, `WithEnergyWeights`, `WithTrustEMAAlpha`,
`WithTrustAggregationWeight`). They form a deliberate, coherent option API, not dead code.
**Not removed.**

Likewise `LiveGossipProbe` / `NewLiveGossipProbe` (`internal/fedgreensub/live_probe.go`) has
zero call sites, but it is the **live-network** adapter — the real-deployment path that the
controlled evaluator deliberately does not exercise — and is described in `README.md`
("The live probe reports only metrics exposed by the dependency and does not fabricate
unavailable counters"). Removing it would delete the library's live-probing capability.
**Not removed.**

---

# KEEP — RESEARCH/REPRODUCIBILITY ARTIFACT

All of the following are **generated but must stay committed**; they are the validated
outputs of the current pipeline:

```
evaluation/results/raw/benchmark_results.csv        (75 observations)
evaluation/results/raw/benchmark_metadata.json
evaluation/results/raw/adaptation_trace.csv         (150 rows)
evaluation/results/processed/benchmark_summary.csv
evaluation/results/processed/statistical_summary.csv
evaluation/results/processed/paired_comparisons.csv
evaluation/results/processed/fedgreen_vs_trust.csv
evaluation/results/processed/analysis_report.md
evaluation/results/plots/                           (7 figures x PNG+PDF)
evaluation/results/analyze_benchmark.py             (source)
```

Also kept: `evaluation/comparison/plots/output/` (14 tracked figures + 3 SVGs) — the
individually-tracked originals that A8's redundant `.zip` duplicates.

# KEEP — TEST/VALIDATION

All `*_test.go` files across `internal/fedgreensub` (23 test files),
`evaluation/comparison/implementations`, `evaluation/comparison`, and
`evaluation/comparison/tests`. Every `Test*` / `Benchmark*` function shows a reference count
of 1 by construction (the Go tooling invokes them by reflection, not by call site); none is
dead code.

---

# Duplicate-functionality review (confirmed NOT consolidated)

Per the "confirm semantics are identical first" instruction, these look similar but were
verified **semantically different** and are deliberately left alone:

| Pair | Verdict |
| --- | --- |
| `results.go:stats()` vs `raw_summary.go:computeStat()` | **DO NOT MERGE.** `stats()` computes **population** standard deviation (÷ n); `computeStat()` computes **sample** standard deviation (÷ n−1, ddof=1) and carries an explicit comment saying so. Merging them would change published numbers in one of the two outputs. |
| `results/energy-scaling/` vs `results/energy_scaling/` | Not duplicates — per-run JSON vs aggregated report (see B2). |
| `energy_report.go` CSV writers vs `raw_dataset.go`/`raw_summary.go` writers | Different schemas and different consumers. No consolidation. |
| `injectTrust` vs `recordTrustMetrics` | Genuine duplicate; the dead one is removed (A3), the live one untouched. |

# Proposed refactors (formatting only)

| File | Current | Proposed | Risk | Behaviour impact |
| --- | --- | --- | --- | --- |
| `evaluation/comparison/implementations/implementation_test.go` | 3 misaligned trailing comments | `gofmt` alignment | none | none (whitespace) |
| `test.go`, `original_gossipsub_benchmarks_test.go`, `internal/fedgreensub/live_probe.go` | CRLF line endings — the **only 3 CRLF files** in the repo; all other 60 Go files are LF | `gofmt` normalises to LF, matching the rest of the repo | none | none (whitespace); git will show these as fully rewritten, which is cosmetic |

No other refactor is proposed. No function was renamed, merged, moved, or re-signatured.

---

# Explicitly out of scope (Phase 5)

The known floating-point nondeterminism in FL aggregation — `FederatedCoordinator.peers` is a
Go map, `RunRound` iterates it, Go randomises map iteration order per process, and
floating-point addition is not associative, producing ULP-level differences in model-derived
fields such as `deployed_gossip_factor` — is **documented and intentionally not fixed**.
No change in this audit touches `coordinator.go`, aggregation order, or numeric precision.
The exported CSV remains reproducible at its written `%.10g` precision.

---

# Summary of applied cleanup

**8 items removed:** 5 dead functions/methods (A1–A5), 1 no-op statement (A6),
2 redundant tracked artifacts (A7–A8, ~629 KB).
**4 files gofmt-normalised.**
**5 items flagged for your review, none touched** (B1–B5).
**Nothing in `evaluation/results/` altered by hand.** No algorithm, evaluator, guardrail, FL,
trust, seed, configuration, or schema change.

One consequential edit followed from A4: removing `finalize` left `import "math"` unused in
`evaluation/comparison/metrics.go`, which Go rejects at compile time. The import was removed
with it (an explicitly allowed cleanup category).

`.gitignore` gained `__pycache__/` and `*.py[cod]` so A7 cannot recur.

---

# Post-cleanup verification (Phases 6-7)

## Before / after

| Check | Before | After |
| --- | --- | --- |
| `go build ./...` | exit 0 | exit 0 |
| `go test ./...` | all `ok` | all `ok` |
| `go vet ./...` | 1 finding (self-assignment) | **0 findings — clean** |
| `gofmt -l` | 4 files | **0 files** |
| Source lines | — | **76 deleted, 14 inserted** across 5 files |

## Benchmark regeneration

`go run ./evaluation/comparison -export-dataset` reported exactly the required counts:

```
Raw benchmark rows: 75      Implementations: 3      Peer counts: 5      Seeds: 5
Missing combinations: 0     Duplicate run IDs: 0
Adaptation trace: 150 rows  Summary: 15 rows
```

## Metric-change check — none

Regenerated outputs were diffed against the pre-cleanup copies:

| File | Result |
| --- | --- |
| `raw/benchmark_results.csv` | **byte-identical** |
| `raw/adaptation_trace.csv` | **byte-identical** |
| `processed/benchmark_summary.csv` | **byte-identical** |
| `processed/statistical_summary.csv` | **byte-identical** |
| `processed/paired_comparisons.csv` | **byte-identical** |
| `processed/fedgreen_vs_trust.csv` | **byte-identical** |
| all 7 `plots/*.png` | **byte-identical** (not even listed as modified by git) |

Delivery ratio, duplicate ratio, latency, resource cost, and cost per delivered message are
therefore unchanged — not merely "within tolerance", but bit-for-bit.

Two files differ, both by provenance metadata only:

- `raw/benchmark_metadata.json` — 2 lines: `generated_at` timestamp and `git_commit`
  (`e0128f2` → `9e29329`; the committed dataset had been exported from an older commit, so
  this is a provenance correction, not a data change).
- `processed/analysis_report.md` — 1 line: the same timestamp/commit provenance line. Every
  reported statistic is unchanged.
- the 7 `plots/*.pdf` files — verified byte-by-byte: each is the same size as before and
  differs in exactly **5 bytes**, all inside the PDF's embedded
  `/CreationDate (D:20260910205134+05'1800')` → `(D:20260910211815+05'1800')` field.
  The rendered content is identical.

## Python pipeline

`python evaluation/results/analyze_benchmark.py` re-ran end to end (Python 3.12.10,
pandas 3.0.5, numpy 2.5.3, scipy 1.18.1, matplotlib 3.11.1):

- raw dataset verified: 75 rows, 3 implementations, 5 peer counts, 5 seeds, no gaps/duplicates
- 15 group means cross-checked against the Go-produced summary: **all means match**
- manual spot check (FedGreenSub, peers=10, resource cost) reproduced exactly:
  mean 67.24954565, sample sd 0.2017424855
- 150 adaptation-trace rows loaded, 7 figures regenerated
- final self-check confirmed the raw dataset unchanged (75 rows still present)

No hard-coded benchmark values were introduced; the script still derives everything from
`benchmark_results.csv`.

## Phase 5 compliance

`internal/fedgreensub/coordinator.go` was **not modified**. Aggregation order, map iteration,
and numeric precision are untouched; the documented ULP-level nondeterminism remains as-is.
(It did not surface in this regeneration — `deployed_gossip_factor` matched byte-for-byte,
consistent with it rounding away at the `%.10g` written precision.)
