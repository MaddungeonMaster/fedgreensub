# FedGreenSub Phases 14-15 Test Runner
# Run this script to see benchmarks and hardening tests

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  FedGreenSub: Benchmarks & Hardening" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Set location to project root
Push-Location $PSScriptRoot

Write-Host "Phase 14: Running benchmarks..." -ForegroundColor Yellow
Write-Host ""

# Run benchmarks only (skip unit tests)
go test ./internal/fedgreensub -bench=. -benchmem -run=^$

Write-Host ""
Write-Host "Phase 15: Running hardening tests..." -ForegroundColor Yellow
Write-Host ""

# Run hardening tests
go test ./internal/fedgreensub -run "Race|Stress|Hardened" -v

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  Phases 14 & 15 testing complete" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green

Pop-Location
