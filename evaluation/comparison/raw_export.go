package main

// raw_export.go is the orchestration entry point for the reproducible raw
// dataset export: run the existing benchmark pipeline, flatten to per-seed
// rows, validate, write the raw CSV + metadata JSON + summary CSV, and print
// a validation report.

import "fmt"

const (
	rawResultsPath         = "evaluation/results/raw/benchmark_results.csv"
	rawMetadataPath        = "evaluation/results/raw/benchmark_metadata.json"
	rawAdaptationTracePath = "evaluation/results/raw/adaptation_trace.csv"
	processedSummary       = "evaluation/results/processed/benchmark_summary.csv"
)

// exportBenchmarkDataset runs the validated benchmark configuration, writes
// the raw per-run dataset, its reproducibility metadata, and the aggregated
// summary dataset, then prints a validation report.
func exportBenchmarkDataset() error {
	rows, cfg, err := generateRawRows()
	if err != nil {
		return fmt.Errorf("generate raw rows: %w", err)
	}

	report := validateRawDataset(rows, cfg)
	printValidationReport(report)

	if err := writeRawCSV(rawResultsPath, rows); err != nil {
		return fmt.Errorf("write raw csv: %w", err)
	}
	if err := writeMetadataJSON(rawMetadataPath, buildDatasetMetadata(cfg)); err != nil {
		return fmt.Errorf("write metadata json: %w", err)
	}
	summary := computeSummary(rows)
	if err := writeSummaryCSV(processedSummary, summary); err != nil {
		return fmt.Errorf("write summary csv: %w", err)
	}

	traceRows, err := generateAdaptationTraceRows()
	if err != nil {
		return fmt.Errorf("generate adaptation trace rows: %w", err)
	}
	if err := writeAdaptationTraceCSV(rawAdaptationTracePath, traceRows); err != nil {
		return fmt.Errorf("write adaptation trace csv: %w", err)
	}

	fmt.Println()
	fmt.Printf("Raw results:      %s\n", rawResultsPath)
	fmt.Printf("Metadata:         %s\n", rawMetadataPath)
	fmt.Printf("Adaptation trace: %s (%d rows)\n", rawAdaptationTracePath, len(traceRows))
	fmt.Printf("Summary:          %s\n", processedSummary)

	if len(report.MissingCombinations) > 0 || len(report.DuplicateRunIDs) > 0 {
		return fmt.Errorf("dataset validation failed: %d missing combinations, %d duplicate run IDs", len(report.MissingCombinations), len(report.DuplicateRunIDs))
	}
	return nil
}

func printValidationReport(report DatasetValidationReport) {
	fmt.Println("Raw benchmark dataset validation")
	fmt.Printf("Raw benchmark rows: %d\n", report.TotalRows)
	fmt.Printf("Implementations: %d\n", report.Implementations)
	fmt.Printf("Peer counts: %d\n", report.PeerCounts)
	fmt.Printf("Seeds: %d\n", report.Seeds)
	fmt.Println()
	fmt.Printf("Missing combinations: %d\n", len(report.MissingCombinations))
	for _, id := range report.MissingCombinations {
		fmt.Printf("  missing: %s\n", id)
	}
	fmt.Printf("Duplicate run IDs: %d\n", len(report.DuplicateRunIDs))
	for _, id := range report.DuplicateRunIDs {
		fmt.Printf("  duplicate: %s\n", id)
	}
}
