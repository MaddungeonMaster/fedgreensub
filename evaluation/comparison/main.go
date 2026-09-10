package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	cfg := DefaultConfig()
	analyze := false
	peerCounts := ""
	exportDataset := false
	flag.BoolVar(&analyze, "analyze", false, "print repetition statistics and percentage comparisons")
	flag.StringVar(&cfg.Scenario, "scenario", cfg.Scenario, "scenario: normal, scaling, highload, packetloss, duplicate, churn, trust, fl")
	flag.IntVar(&cfg.Peers, "peers", cfg.Peers, "number of peers")
	flag.IntVar(&cfg.Messages, "messages", cfg.Messages, "number of messages")
	flag.IntVar(&cfg.MessageSize, "message-size", cfg.MessageSize, "message size in bytes")
	flag.Float64Var(&cfg.PublishRate, "rate", cfg.PublishRate, "publish rate per second")
	flag.Int64Var(&cfg.Seed, "seed", cfg.Seed, "deterministic random seed")
	flag.IntVar(&cfg.Repetitions, "repetitions", cfg.Repetitions, "independent repetitions")
	flag.StringVar(&peerCounts, "peer-counts", "", "comma-separated peer counts for multi-size experiments")
	flag.BoolVar(&exportDataset, "export-dataset", false, "generate the reproducible raw+processed benchmark dataset (evaluation/results/raw, evaluation/results/processed) using the fixed validated configuration; ignores other flags")
	flag.Parse()
	if exportDataset {
		if err := exportBenchmarkDataset(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if peerCounts != "" {
		cfg.PeerCounts = parsePeerCounts(peerCounts)
	}
	cfg = cfg.normalize()
	results, err := runExperiments(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root := filepath.Join("evaluation", "comparison", "results")
	for _, result := range results {
		if _, err := saveResult(root, result); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := writeCSV(filepath.Join(root, "summary.csv"), results); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.Scenario == "energy-scaling" {
		if err := writeEnergyScalingReport(filepath.Join(root, "energy_scaling"), results); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if data, err := json.MarshalIndent(summarize(results), "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(root, cfg.Scenario, "statistics.json"), data, 0644)
	}
	printSummary(results, cfg)
	if cfg.Scenario != "energy-scaling" {
		printFLTrace(results, cfg.Seed)
	}
	if analyze {
		printAnalysis(results)
	}
}

func parsePeerCounts(value string) []int {
	counts := make([]int, 0)
	for _, part := range strings.Split(value, ",") {
		peers, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && peers > 0 {
			counts = append(counts, peers)
		}
	}
	return counts
}
