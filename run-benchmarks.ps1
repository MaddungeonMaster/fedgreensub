# FedGreenSub Benchmarks Runner
# Runs all benchmarks and displays normalized results in tabular format

$ErrorActionPreference = 'Stop'

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "       FedGreenSub Benchmarks" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

Push-Location $PSScriptRoot

Write-Host "Running benchmarks..." -ForegroundColor Yellow
Write-Host ""

$benchmarkOutput = & go test ./internal/fedgreensub -run '^$' -bench '.' -benchmem 2>&1
$exitCode = $LASTEXITCODE

if ($exitCode -eq 0) {
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host "       Benchmarks completed successfully" -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host ""

    $results = @()

    foreach ($line in $benchmarkOutput) {
        if ($line -notmatch '^\s*Benchmark\S+') {
            continue
        }

        $match = [regex]::Match($line, '^(\s*Benchmark\S+)\s+(\d+)\s+(\d+(?:\.\d+)?)\s*(ns/op|us/op|µs/op|ms/op|s/op)\s+(\d+(?:\.\d+)?)\s*(B/op|KB/op|kB/op|MB/op|GB/op)\s+(\d+(?:\.\d+)?)\s*allocs/op')

        if (-not $match.Success) {
            continue
        }

        $benchmarkName = $match.Groups[1].Value.Trim()
        $iterations = [long]::Parse($match.Groups[2].Value)
        $timeValue = [double]::Parse($match.Groups[3].Value)
        $timeUnit = $match.Groups[4].Value
        $bytesValue = [double]::Parse($match.Groups[5].Value)
        $bytesUnit = $match.Groups[6].Value
        $allocsValue = [double]::Parse($match.Groups[7].Value)

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
            Benchmark   = $benchmarkName
            Iterations  = $iterations
            'ns/op'     = [math]::Round($nsPerOp, 2)
            'B/op'      = [math]::Round($bytesPerOp, 2)
            'allocs/op' = [math]::Round($allocsValue, 2)
        }
    }

    if ($results.Count -gt 0) {
        Write-Host "Benchmark Results (Normalized)" -ForegroundColor Cyan
        Write-Host ""
        $results | Format-Table -AutoSize

        Write-Host ""
        Write-Host "==========================================" -ForegroundColor Cyan
        Write-Host "                 Legend" -ForegroundColor Cyan
        Write-Host "==========================================" -ForegroundColor Cyan
        Write-Host "ns/op     = nanoseconds per operation" -ForegroundColor Gray
        Write-Host "B/op      = bytes allocated per operation" -ForegroundColor Gray
        Write-Host "allocs/op = allocations per operation" -ForegroundColor Gray
        Write-Host "Iterations = number of benchmark iterations" -ForegroundColor Gray
        Write-Host ""
        Write-Host "Lower ns/op, B/op and allocs/op are generally better." -ForegroundColor Yellow
    }
    else {
        Write-Host "No benchmark results could be parsed." -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Raw benchmark output:" -ForegroundColor Gray
        $benchmarkOutput
    }
}
else {
    Write-Host "==========================================" -ForegroundColor Red
    Write-Host "       Benchmarks failed" -ForegroundColor Red
    Write-Host "       Exit code: $exitCode" -ForegroundColor Red
    Write-Host "==========================================" -ForegroundColor Red
    Write-Host ""

    Write-Host "Raw output:" -ForegroundColor Yellow
    $benchmarkOutput
}

Write-Host ""
Pop-Location
