# Comparative Benchmark: Normal GossipSub vs Federated GossipSub
# This script runs the actual benchmark suite and compares the measured numbers.

$ErrorActionPreference = 'Stop'

function Parse-BenchmarkLines {
    param(
        [string[]]$Lines,
        [string[]]$AllowedNames
    )

    $results = @()

    foreach ($line in $Lines) {
        if ($line -notmatch '^\s*Benchmark\S+') {
            continue
        }

        $match = [regex]::Match($line, '^(\s*Benchmark\S+)\s+(\d+)\s+(\d+(?:\.\d+)?)\s*(ns/op|us/op|µs/op|ms/op|s/op)\s+(\d+(?:\.\d+)?)\s*(B/op|KB/op|kB/op|MB/op|GB/op)\s+(\d+(?:\.\d+)?)\s*allocs/op')
        if (-not $match.Success) { continue }

        $name = $match.Groups[1].Value.Trim()
        $baseName = $name -replace '-\d+$', ''
        if ($AllowedNames -and $baseName -notin $AllowedNames) { continue }

        $timeValue = [double]::Parse($match.Groups[3].Value)
        $timeUnit = $match.Groups[4].Value
        $bytesValue = [double]::Parse($match.Groups[5].Value)
        $bytesUnit = $match.Groups[6].Value
        $allocs = [double]::Parse($match.Groups[7].Value)

        switch ($timeUnit) {
            'ns/op' { $nsPerOp = $timeValue }
            'us/op' { $nsPerOp = $timeValue * 1000 }
            'µs/op' { $nsPerOp = $timeValue * 1000 }
            'ms/op' { $nsPerOp = $timeValue * 1000000 }
            's/op' { $nsPerOp = $timeValue * 1000000000 }
            default { $nsPerOp = $timeValue }
        }

        switch ($bytesUnit) {
            'B/op' { $bytesPerOp = $bytesValue }
            'KB/op' { $bytesPerOp = $bytesValue * 1024 }
            'kB/op' { $bytesPerOp = $bytesValue * 1000 }
            'MB/op' { $bytesPerOp = $bytesValue * 1024 * 1024 }
            'GB/op' { $bytesPerOp = $bytesValue * 1024 * 1024 * 1024 }
            default { $bytesPerOp = $bytesValue }
        }

        $results += [PSCustomObject]@{
            Name        = $baseName
            TimeNsPerOp = [math]::Round($nsPerOp, 2)
            BytesPerOp  = [math]::Round($bytesPerOp, 2)
            AllocsPerOp = [math]::Round($allocs, 2)
        }
    }

    return $results
}

Write-Host "===========================================" -ForegroundColor Cyan
Write-Host "Normal GossipSub vs Federated GossipSub" -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan
Write-Host ""

Push-Location $PSScriptRoot

$fedNames = @(
    'BenchmarkCollectorBaseline',
    'BenchmarkCollectorConcurrentReads',
    'BenchmarkParameterValidation',
    'BenchmarkFullRuntimeCycle'
)

$origNames = @(
    'BenchmarkOriginalGossipSubPublish',
    'BenchmarkOriginalGossipSubConcurrentPublish'
)

Write-Host "Running actual FedGreenSub benchmark suite..." -ForegroundColor Yellow
$fedOutput = & go test ./internal/fedgreensub -run '^$' -bench 'CollectorBaseline|CollectorConcurrentReads|FullRuntimeCycle|ParameterValidation' -benchmem 2>&1
$fedExit = $LASTEXITCODE
if ($fedExit -ne 0) {
    Write-Error "FedGreenSub benchmark run failed with exit code $fedExit"
}

Write-Host "Running actual original GossipSub benchmark suite..." -ForegroundColor Yellow
$origOutput = & go test . -run '^$' -bench 'OriginalGossipSub' -benchmem 2>&1
$origExit = $LASTEXITCODE
if ($origExit -ne 0) {
    Write-Error "Original GossipSub benchmark run failed with exit code $origExit"
}

$fedResults = Parse-BenchmarkLines -Lines $fedOutput -AllowedNames $fedNames
$origResults = Parse-BenchmarkLines -Lines $origOutput -AllowedNames $origNames

$comparison = @(
    $origResults | Select-Object @{Name='Implementation';Expression={'Normal GossipSub'}}, @{Name='Benchmark';Expression={$_.Name}}, @{Name='ns/op';Expression={$_.TimeNsPerOp}}, @{Name='B/op';Expression={$_.BytesPerOp}}, @{Name='allocs/op';Expression={$_.AllocsPerOp}}, @{Name='Notes';Expression={'Network publish path'}}
    $fedResults | Select-Object @{Name='Implementation';Expression={'FedGreenSub'}}, @{Name='Benchmark';Expression={$_.Name}}, @{Name='ns/op';Expression={$_.TimeNsPerOp}}, @{Name='B/op';Expression={$_.BytesPerOp}}, @{Name='allocs/op';Expression={$_.AllocsPerOp}}, @{Name='Notes';Expression={'Adaptive runtime path'}}
)

Write-Host ""
Write-Host "Measured results:" -ForegroundColor Cyan
$comparison | Format-Table -AutoSize

Write-Host ""
Write-Host "Interpretation:" -ForegroundColor Yellow
Write-Host "  - The standard libp2p GossipSub publish benchmarks are network-level publish operations and measure message propagation cost." -ForegroundColor White
Write-Host "  - The FedGreenSub benchmarks measure the adaptive runtime and tuning loop, not a single message publish path." -ForegroundColor White
Write-Host "  - In this repository, the adaptive runtime adds measurable control-plane overhead, but it provides live optimization and resource-aware behavior." -ForegroundColor White
Write-Host ""
Write-Host "The comparison above is based on actual benchmark output from the current workspace." -ForegroundColor Green
Write-Host ""

Pop-Location
