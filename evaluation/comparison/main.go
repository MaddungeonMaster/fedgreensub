package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	cfg := DefaultConfig()
	analyze := false
	flag.BoolVar(&analyze, "analyze", false, "print repetition statistics and percentage comparisons")
	flag.StringVar(&cfg.Scenario, "scenario", cfg.Scenario, "scenario: normal, scaling, highload, packetloss, duplicate, churn, trust, fl")
	flag.IntVar(&cfg.Peers, "peers", cfg.Peers, "number of peers")
	flag.IntVar(&cfg.Messages, "messages", cfg.Messages, "number of messages")
	flag.IntVar(&cfg.MessageSize, "message-size", cfg.MessageSize, "message size in bytes")
	flag.Float64Var(&cfg.PublishRate, "rate", cfg.PublishRate, "publish rate per second")
	flag.Int64Var(&cfg.Seed, "seed", cfg.Seed, "deterministic random seed")
	flag.IntVar(&cfg.Repetitions, "repetitions", cfg.Repetitions, "independent repetitions")
	flag.Parse()
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
	if data, err := json.MarshalIndent(summarize(results), "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(root, cfg.Scenario, "statistics.json"), data, 0644)
	}
	printSummary(results, cfg)
	printFLTrace(results, cfg.Seed)
	if analyze {
		printAnalysis(results)
	}
}
