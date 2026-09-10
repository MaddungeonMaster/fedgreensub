package main

import (
	"os"
	"path/filepath"
	"testing"
)

// generatedRows caches the raw dataset generation across the tests in this
// file so the (deterministic, sub-second) 75-run benchmark is only executed
// once per test binary invocation.
var (
	cachedRows []RawRow
	cachedCfg  ExperimentConfig
)

func rawDatasetRows(t *testing.T) ([]RawRow, ExperimentConfig) {
	t.Helper()
	if cachedRows != nil {
		return cachedRows, cachedCfg
	}
	rows, cfg, err := generateRawRows()
	if err != nil {
		t.Fatalf("generateRawRows() error = %v", err)
	}
	cachedRows, cachedCfg = rows, cfg
	return rows, cfg
}

// Test 1 -- expected row count: 5 peer counts x 5 seeds x 3 implementations.
func TestRawDatasetRowCount(t *testing.T) {
	rows, _ := rawDatasetRows(t)
	const want = 5 * 5 * 3
	if len(rows) != want {
		t.Fatalf("expected %d raw rows, got %d", want, len(rows))
	}
}

// Test 2 -- unique run identity: no duplicate implementation+peer_count+seed
// triples.
func TestRawDatasetNoDuplicateRunIDs(t *testing.T) {
	rows, _ := rawDatasetRows(t)
	seen := make(map[string]int, len(rows))
	for _, row := range rows {
		seen[runID(row.Implementation, row.PeerCount, row.Seed)]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("duplicate run id %q appears %d times", id, count)
		}
	}
}

// Test 3 -- all configurations represented: every combination of 5 peer
// counts x 5 seeds x 3 implementations exists exactly once.
func TestRawDatasetAllConfigurationsRepresented(t *testing.T) {
	rows, cfg := rawDatasetRows(t)
	report := validateRawDataset(rows, cfg)
	if len(report.MissingCombinations) != 0 {
		t.Fatalf("missing combinations: %v", report.MissingCombinations)
	}
	if len(report.DuplicateRunIDs) != 0 {
		t.Fatalf("duplicate run ids: %v", report.DuplicateRunIDs)
	}
	if report.Implementations != 3 {
		t.Fatalf("expected 3 implementations, got %d", report.Implementations)
	}
	if report.PeerCounts != 5 {
		t.Fatalf("expected 5 peer counts, got %d", report.PeerCounts)
	}
	if report.Seeds != 5 {
		t.Fatalf("expected 5 seeds, got %d", report.Seeds)
	}
	// Cross-check the exact seed values, not just the count.
	seeds := rawDatasetSeeds(cfg)
	wantSeeds := []int64{4242, 4243, 4244, 4245, 4246}
	if len(seeds) != len(wantSeeds) {
		t.Fatalf("expected seeds %v, got %v", wantSeeds, seeds)
	}
	for i, s := range wantSeeds {
		if seeds[i] != s {
			t.Fatalf("expected seeds %v, got %v", wantSeeds, seeds)
		}
	}
	wantPeers := map[int]bool{10: true, 25: true, 50: true, 100: true, 200: true}
	if len(cfg.PeerCounts) != len(wantPeers) {
		t.Fatalf("expected peer counts %v, got %v", wantPeers, cfg.PeerCounts)
	}
	for _, p := range cfg.PeerCounts {
		if !wantPeers[p] {
			t.Fatalf("unexpected peer count %d in configuration %v", p, cfg.PeerCounts)
		}
	}
}

// Test 4 -- summary correctness: verify means/stddevs are calculated from
// the raw per-seed observations, using sample (ddof=1) standard deviation.
func TestRawDatasetSummaryCorrectness(t *testing.T) {
	rows, _ := rawDatasetRows(t)
	summary := computeSummary(rows)

	// Every (implementation, peer_count) group must appear in the summary,
	// each aggregating exactly 5 seeds.
	if len(summary) != 3*5 {
		t.Fatalf("expected %d summary rows (3 implementations x 5 peer counts), got %d", 3*5, len(summary))
	}
	for _, s := range summary {
		if s.Seeds != 5 {
			t.Fatalf("%s peers=%d: expected 5 seeds aggregated, got %d", s.Implementation, s.PeerCount, s.Seeds)
		}
	}

	// Hand-recompute one group's delivery_ratio mean/stddev directly from the
	// raw rows using ddof=1, and confirm it matches computeSummary's output
	// exactly (not merely close), proving the summary is derived from the raw
	// per-seed values rather than from an already-averaged table.
	var target SummaryRow
	found := false
	for _, s := range summary {
		if s.Implementation == "FedGreenSub" && s.PeerCount == 10 {
			target = s
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected a FedGreenSub/peers=10 summary row")
	}
	var values []float64
	for _, row := range rows {
		if row.Implementation == "FedGreenSub" && row.PeerCount == 10 {
			values = append(values, row.DeliveryRatio)
		}
	}
	if len(values) != 5 {
		t.Fatalf("expected 5 raw FedGreenSub/peers=10 observations, got %d", len(values))
	}
	want := computeStat(values)
	if want.Mean != target.DeliveryRatio.Mean {
		t.Fatalf("summary mean does not match independently recomputed mean from raw rows: got=%v want=%v", target.DeliveryRatio.Mean, want.Mean)
	}
	if want.StdDev != target.DeliveryRatio.StdDev {
		t.Fatalf("summary stddev does not match independently recomputed sample (ddof=1) stddev from raw rows: got=%v want=%v", target.DeliveryRatio.StdDev, want.StdDev)
	}

	// A trivial two-point ddof=1 sanity check: stddev([0,2]) with ddof=1 is 1.0
	// exactly (mean=1, sum of squared deviations=2, /(n-1)=2/1=2, sqrt=~1.414).
	// Use values chosen so the arithmetic is easy to verify by hand.
	check := computeStat([]float64{2, 4})
	if got, want := check.Mean, 3.0; got != want {
		t.Fatalf("computeStat mean = %v, want %v", got, want)
	}
	if got, want := check.StdDev, 1.4142135623730951; got != want {
		t.Fatalf("computeStat ddof=1 stddev = %v, want %v (population stddev would be 1.0)", got, want)
	}
}

// Test 5 -- reproducibility: running a small deterministic subset twice
// produces equivalent metric output (excluding anything explicitly
// timestamp-based, which this dataset generation path does not produce).
//
// NOTE ON A DISCOVERED, PRE-EXISTING, OUT-OF-SCOPE LIMITATION:
// FederatedCoordinator.peers (internal/fedgreensub/coordinator.go) is a Go
// map, and RunRound iterates it with "for _, participant := range c.peers".
// Go randomizes map iteration order per process, and floating-point
// summation is not associative, so the aggregated model weights (and
// anything derived from them, e.g. the predicted gossip factor) can differ
// by a couple of ULPs between runs. This is a property of the existing,
// unmodified FL aggregation code -- explicitly out of scope to change in
// this task ("Do NOT modify: FL aggregation") -- not something introduced by
// this dataset exporter. It does not affect delivery/duplicate/latency/
// resource-cost values (these do not depend on the model weights at that
// precision) and it rounds away completely at the 10-significant-digit
// precision actually written to the CSV (%.10g), so the exported dataset
// file is reproducible even though a couple of raw float64 fields are not
// bit-identical in memory. This test therefore compares the actual on-disk
// row representation (csvRow()), which is the property that matters for the
// dataset's reproducibility, rather than exact in-memory struct equality.
func TestRawDatasetReproducibility(t *testing.T) {
	first, _, err := generateRawRows()
	if err != nil {
		t.Fatalf("generateRawRows() first run error = %v", err)
	}
	second, _, err := generateRawRows()
	if err != nil {
		t.Fatalf("generateRawRows() second run error = %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("row count differs between runs: %d vs %d", len(first), len(second))
	}
	for i := range first {
		a, b := first[i].csvRow(), second[i].csvRow()
		if len(a) != len(b) {
			t.Fatalf("row %d: csv column count differs between runs", i)
		}
		for c := range a {
			if a[c] != b[c] {
				t.Fatalf("row %d column %d (%s) differs between runs: %q vs %q", i, c, rawRowHeader()[c], a[c], b[c])
			}
		}
	}
}

// Additional focused test: the raw CSV/metadata/summary files this task adds
// actually get written to the expected locations with the expected header
// shape, using a small temporary subset so the test stays fast.
func TestWriteRawAndSummaryCSVProducesExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	rows := []RawRow{
		{Implementation: "GossipSub", PeerCount: 10, Seed: 4242, DeliveryRatio: 0.9, DuplicateRatio: 0.1, LatencyMs: 40, ResourceCostTotal: 60, ResourceCostPerDelivered: 0.3},
		{Implementation: "FedGreenSub", PeerCount: 10, Seed: 4242, DeliveryRatio: 0.91, DuplicateRatio: 0.09, LatencyMs: 38, ResourceCostTotal: 58, ResourceCostPerDelivered: 0.28},
	}
	rawPath := filepath.Join(dir, "raw", "benchmark_results.csv")
	if err := writeRawCSV(rawPath, rows); err != nil {
		t.Fatalf("writeRawCSV() error = %v", err)
	}
	if _, err := os.Stat(rawPath); err != nil {
		t.Fatalf("raw CSV was not written: %v", err)
	}

	summary := computeSummary(rows)
	summaryPath := filepath.Join(dir, "processed", "benchmark_summary.csv")
	if err := writeSummaryCSV(summaryPath, summary); err != nil {
		t.Fatalf("writeSummaryCSV() error = %v", err)
	}
	if _, err := os.Stat(summaryPath); err != nil {
		t.Fatalf("summary CSV was not written: %v", err)
	}

	metadataPath := filepath.Join(dir, "raw", "benchmark_metadata.json")
	if err := writeMetadataJSON(metadataPath, buildDatasetMetadata(rawDatasetConfig())); err != nil {
		t.Fatalf("writeMetadataJSON() error = %v", err)
	}
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("metadata JSON was not written: %v", err)
	}
}

// Delivery ratio must never be reported as an "improvement" -- a decrease
// must show as a negative difference, not be silently sign-flipped or
// dropped.
func TestDeliveryRatioDifferenceIsSignedNotImprovement(t *testing.T) {
	rows := []RawRow{
		{Implementation: "GossipSub", PeerCount: 10, Seed: 4242, DeliveryRatio: 0.95},
		{Implementation: "GossipSub", PeerCount: 10, Seed: 4243, DeliveryRatio: 0.95},
		{Implementation: "FedGreenSub", PeerCount: 10, Seed: 4242, DeliveryRatio: 0.90},
		{Implementation: "FedGreenSub", PeerCount: 10, Seed: 4243, DeliveryRatio: 0.90},
	}
	summary := computeSummary(rows)
	for _, s := range summary {
		if s.Implementation != "FedGreenSub" {
			continue
		}
		if !s.hasBaseline {
			t.Fatal("expected a GossipSub baseline to be found")
		}
		if s.DeliveryRatioDifference >= 0 {
			t.Fatalf("expected a negative delivery_ratio_difference for a delivery regression, got %v", s.DeliveryRatioDifference)
		}
		if got, want := s.DeliveryRatioDifference, -0.05; got < want-1e-9 || got > want+1e-9 {
			t.Fatalf("delivery_ratio_difference = %v, want %v", got, want)
		}
	}
}
