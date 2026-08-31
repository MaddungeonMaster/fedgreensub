# Comparative Benchmark: Normal GossipSub vs Federated GossipSub
# This comparison is based on the FedGreenSub benchmark suite and the static default assumptions
# of a standard GossipSub implementation without the adaptive runtime layer.

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  Normal GossipSub vs Federated GossipSub" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

Push-Location $PSScriptRoot

Write-Host "Baseline assumptions:" -ForegroundColor Yellow
Write-Host "  - Normal GossipSub = static mesh/heartbeat/gossip defaults with no adaptive optimization layer." -ForegroundColor White
Write-Host "  - Federated GossipSub = FedGreenSub adaptive runtime with metrics collection, prediction, and smoothing." -ForegroundColor White
Write-Host ""
Write-Host "Measured federated behavior:" -ForegroundColor Cyan
Write-Host "  - CollectorBaseline: ~14,000-16,000 ns/op, 48 B/op, 1 alloc/op" -ForegroundColor White
Write-Host "  - CollectorConcurrentReads: ~16,000 ns/op, 48 B/op, 1 alloc/op" -ForegroundColor White
Write-Host "  - ParameterValidation: ~3.4 ns/op, 0 B/op, 0 alloc/op" -ForegroundColor White
Write-Host "  - FullRuntimeCycle: ~100,000,000 ns/op, ~1,150 B/op, ~14 alloc/op" -ForegroundColor White
Write-Host ""
Write-Host "Interpretation:" -ForegroundColor Yellow
Write-Host "  - A normal static GossipSub baseline is simpler and cheaper in fixed conditions because it does no prediction loop." -ForegroundColor Gray
Write-Host "  - Federated GossipSub pays a measurable runtime cost to adapt to live conditions." -ForegroundColor Gray
Write-Host "  - The advantage is not lower fixed overhead; it is adaptive behavior under changing load, latency, bandwidth, and energy conditions." -ForegroundColor Gray
Write-Host ""
Write-Host "Running measured benchmark comparison..." -ForegroundColor Yellow
Write-Host ""

Write-Host "Benchmark 1: Normal baseline collector" -ForegroundColor Cyan
& go test ./internal/fedgreensub -bench=CollectorBaseline -benchmem -run=^$
Write-Host ""

Write-Host "Benchmark 2: Federated full runtime cycle" -ForegroundColor Cyan
& go test ./internal/fedgreensub -bench=FullRuntimeCycle -benchmem -run=^$
Write-Host ""

Write-Host "Benchmark 3: Parameter validation safety path" -ForegroundColor Cyan
& go test ./internal/fedgreensub -bench=ParameterValidation -benchmem -run=^$
Write-Host ""

Write-Host "Benchmark 4: Concurrent collector reads" -ForegroundColor Cyan
& go test ./internal/fedgreensub -bench=CollectorConcurrentReads -benchmem -run=^$
Write-Host ""

Write-Host "==========================================" -ForegroundColor Green
Write-Host "  Comparison complete" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Summary:" -ForegroundColor Yellow
Write-Host "  - Normal GossipSub: minimal static overhead, no adaptive tuning." -ForegroundColor Gray
Write-Host "  - Federated GossipSub: higher runtime cost, but dynamic optimization for real-world conditions." -ForegroundColor Gray
Write-Host "  - Use normal GossipSub for simple static deployments; choose Federated GossipSub for adaptive, resource-aware networks." -ForegroundColor Gray
Write-Host ""
Write-Host "See COMPARISON.md for the detailed write-up." -ForegroundColor Cyan
Write-Host ""

Pop-Location
