#!/bin/bash
# Comparative Benchmark: Normal GossipSub vs FedGreenSub (Bash/Linux/Mac)
#
# This script runs the actual benchmark suites and prints their real, measured
# output. It deliberately reports nothing that "go test -bench" did not produce:
# no summary percentages, no energy figures, and no claims about physical units.
#
# Terminology: FedGreenSub's evaluation reports a modeled, normalized
# resource-cost proxy. It is not a physical energy measurement and is never
# expressed in Joules.
#
# These microbenchmarks measure runtime cost only. For the validated end-to-end
# comparison across peer counts and seeds, use the dataset pipeline instead:
#
#   go run ./evaluation/comparison -export-dataset
#   python evaluation/results/analyze_benchmark.py
#
# which write evaluation/results/{raw,processed,plots}.

set -u

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR" || exit 1

echo "=========================================="
echo "  Normal GossipSub vs FedGreenSub"
echo "=========================================="
echo ""

status=0

echo "Running actual FedGreenSub benchmark suite..."
echo "------------------------------------------"
go test ./internal/fedgreensub -run '^$' -benchmem \
    -bench 'CollectorBaseline|CollectorConcurrentReads|FullRuntimeCycle|ParameterValidation'
fed_exit=$?
if [ $fed_exit -ne 0 ]; then
    echo "FedGreenSub benchmark run failed with exit code $fed_exit" >&2
    status=$fed_exit
fi

echo ""
echo "Running actual original GossipSub benchmark suite..."
echo "------------------------------------------"
go test . -run '^$' -bench 'OriginalGossipSub' -benchmem
orig_exit=$?
if [ $orig_exit -ne 0 ]; then
    echo "Original GossipSub benchmark run failed with exit code $orig_exit" >&2
    status=$orig_exit
fi

echo ""
echo "=========================================="
echo "  Comparison complete"
echo "=========================================="
echo ""
echo "Interpretation:"
echo "  - The original GossipSub benchmarks measure the network-level publish path."
echo "  - The FedGreenSub benchmarks measure the adaptive runtime and tuning loop,"
echo "    not a single message publish path."
echo "  - The two suites measure different things and are not a like-for-like speed"
echo "    comparison; read each set of numbers on its own terms."
echo ""
echo "All numbers above are the actual output of 'go test -bench' in this workspace."
echo ""

exit $status
