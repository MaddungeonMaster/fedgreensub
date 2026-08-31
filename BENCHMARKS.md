# Running FedGreenSub Benchmarks & Tests

Quick scripts to run benchmarks and hardening tests for FedGreenSub phases 14-15.

## Scripts

### 1. **run-benchmarks.ps1** (PowerShell - Windows)
Runs all 16 benchmarks with memory statistics.

```powershell
.\run-benchmarks.ps1
```

### 2. **run-benchmarks.sh** (Bash - Linux/Mac)
Same as above for Unix-like systems.

```bash
chmod +x run-benchmarks.sh
./run-benchmarks.sh
```

### 3. **run-phases-14-15.ps1** (PowerShell - Windows)
Runs Phase 14 benchmarks and Phase 15 hardening tests together.

```powershell
.\run-phases-14-15.ps1
```

## Manual Commands

### Run all benchmarks with memory stats:
```bash
go test ./internal/fedgreensub -bench=. -benchmem
```

### Run benchmarks only (skip unit tests):
```bash
go test ./internal/fedgreensub -bench=. -benchmem -run=^$
```

### Run hardening tests only:
```bash
go test ./internal/fedgreensub -run "Race|Stress|Hardened" -v
```

### Run all tests (unit + benchmarks + hardening):
```bash
go test ./internal/fedgreensub -v
```

### Run specific benchmark:
```bash
go test ./internal/fedgreensub -bench=CollectorBaseline -benchmem
```

## Understanding Benchmark Output

Each benchmark line shows:
- **BenchmarkName** - Name of the benchmark
- **N** - Number of iterations completed
- **ns/op** - Nanoseconds per operation (lower is better)
- **B/op** - Bytes allocated per operation (lower is better)
- **allocs/op** - Number of allocations per operation (lower is better)

Example output:
```
BenchmarkCollectorBaseline-8         1000000    1234 ns/op    256 B/op   2 allocs/op
```

This means:
- Ran 1,000,000 iterations
- Average 1234 nanoseconds per operation
- Allocated 256 bytes per operation
- Made 2 allocations per operation

## Benchmarks Included (Phase 14)

**Component Baselines:**
- CollectorBaseline
- EnergyEstimatorBaseline
- HeuristicPredictorBaseline
- ParameterValidation
- ParameterSmoothing

**Feature Benchmarks:**
- LocalTrainerTraining
- FedAvgAggregation

**Runtime Benchmarks:**
- RuntimeMetricsLoop
- FullRuntimeCycle

**Config Benchmarks:**
- DefaultConfigCreation
- NewConfigWithOptions
- SnapshotStoreUpdateAndRead

**Advanced:**
- ParameterSmootherConvergence
- ConcurrentParameterManagerReads
- CollectorConcurrentReads

## Hardening Tests Included (Phase 15)

**Race Conditions (4 tests):**
- TestRaceConditionConcurrentParameterApplication
- TestRaceConditionRuntimeConcurrentStatus
- TestRaceConditionSnapshotStoreConcurrentAccess
- TestRaceConditionParameterSmootherConcurrentSmoothing

**Stress Testing (1 test):**
- TestStressRuntimeStartStopCycles

**Nil-Safety (1 test):**
- TestHardenedErrorHandlingNilSafety

**Boundary Conditions (1 test):**
- TestHardenedBoundaryConditions

**Config Hardening (1 test):**
- TestHardenedConfigNormalization

**Memory/Lifecycle (3 tests):**
- TestHardenedMemoryLeakDetection
- TestHardenedContextCancellation
- TestHardenedPanicRecovery

## Performance Notes

- Benchmarks typically run for 1 second per benchmark by default
- Use `-benchtime=3s` for longer runs with more iterations
- Use `-benchtime=100ms` for quicker runs (less stable results)
- Memory profiling: `go test ./internal/fedgreensub -bench=. -memprofile=mem.prof`
- CPU profiling: `go test ./internal/fedgreensub -bench=. -cpuprofile=cpu.prof`

## Analyzing Results

After profiling, analyze with:
```bash
go tool pprof mem.prof
go tool pprof cpu.prof
```

Type `top` to see the top functions, `list <function>` to see source-level details.
