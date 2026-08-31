#!/bin/bash
# FedGreenSub Benchmarks Runner (Bash/Linux/Mac)
# Run this script to execute all benchmarks and display results

echo "=========================================="
echo "  FedGreenSub Benchmarks"
echo "=========================================="
echo ""

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

echo "Running benchmarks..."
echo ""

# Run benchmarks with memory stats
go test ./internal/fedgreensub -bench=. -benchmem

EXIT_CODE=$?
echo ""

if [ $EXIT_CODE -eq 0 ]; then
    echo "=========================================="
    echo "  ✓ Benchmarks completed successfully"
    echo "=========================================="
else
    echo "=========================================="
    echo "  ✗ Benchmarks failed with exit code: $EXIT_CODE"
    echo "=========================================="
fi

echo ""
echo "Quick reference:"
echo "  ns/op   = nanoseconds per operation"
echo "  B/op    = bytes allocated per operation"
echo "  allocs/op = number of allocations per operation"
echo ""
