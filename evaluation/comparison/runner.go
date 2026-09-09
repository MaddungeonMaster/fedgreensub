package main

import (
	"fmt"
	"time"

	"golang_project/evaluation/comparison/implementations"
)

func newImplementation(name string) implementations.Implementation {
	switch name {
	case "gossipsub":
		return &implementations.BaselineGossipSub{}
	case "heuristic":
		return &implementations.HeuristicGossipSub{}
	case "fl":
		return &implementations.FederatedGossipSub{}
	case "fedgreen":
		return &implementations.FedGreenSub{}
	default:
		return &implementations.TrustAwareFedGreenSub{}
	}
}

func implementationNames() []string {
	return []string{"gossipsub", "heuristic", "fl", "fedgreen", "trustaware"}
}

func runExperiments(cfg ExperimentConfig) ([]Result, error) {
	cfg = scenarioConfig(cfg).normalize()
	results := make([]Result, 0, cfg.Repetitions*5)
	for repetition := 1; repetition <= cfg.Repetitions; repetition++ {
		for _, name := range implementationNames() {
			impl := newImplementation(name)
			if err := impl.Start(implementations.Config{FLRounds: cfg.FLRounds}); err != nil {
				return nil, fmt.Errorf("start %s: %w", name, err)
			}
			workload := cfg.Workload()
			workload.RandomSeed += int64(repetition - 1)
			if err := impl.RunWorkload(implementations.Workload{Peers: workload.Peers, Messages: workload.Messages, MessageSize: workload.MessageSize, PublishRate: workload.PublishRate, Duration: workload.Duration, RandomSeed: workload.RandomSeed, PacketLoss: workload.PacketLoss, Latency: workload.Latency, Duplicates: workload.Duplicates, Churn: workload.Churn}); err != nil {
				return nil, fmt.Errorf("run %s: %w", name, err)
			}
			metrics := impl.Metrics()
			if err := impl.Stop(); err != nil {
				return nil, err
			}
			results = append(results, Result{ExperimentID: fmt.Sprintf("%d-%d", time.Now().UnixNano(), repetition), Timestamp: time.Now().UTC(), Scenario: cfg.Scenario, Implementation: name, PeerCount: cfg.Peers, MessageCount: cfg.Messages, MessageSize: cfg.MessageSize, PublishRate: cfg.PublishRate, Duration: cfg.Duration.String(), Seed: workload.RandomSeed, Repetition: repetition, Metrics: convertMetrics(metrics)})
		}
	}
	return results, nil
}

func convertMetrics(m implementations.Metrics) ExperimentMetrics {
	return ExperimentMetrics{CPU: m.CPU, MemoryMB: m.MemoryMB, BytesSent: m.BytesSent, BytesReceived: m.BytesReceived, Published: m.Published, Received: m.Received, Delivered: m.Delivered, Duplicates: m.Duplicates, DeliveryRatio: m.DeliveryRatio, DuplicateRatio: m.DuplicateRatio, LatencyAverage: m.LatencyAverage, LatencyP95: m.LatencyP95, AverageMeshDegree: m.MeshDegree, HeartbeatCount: m.HeartbeatCount, Energy: m.Energy, EnergyPerDeliveredMessage: m.EnergyPerDeliveredMessage, AverageTrust: m.AverageTrust, MinimumTrust: m.MinimumTrust, TrustVariance: m.TrustVariance, TrustedPeerRatio: m.TrustedPeerRatio, TrustUpdateTime: m.TrustUpdateTime, PeerRankingTime: m.PeerRankingTime, TrustAggregationTime: m.TrustAggregationTime, FLRounds: m.FLRounds, TrainingLoss: m.TrainingLoss, GlobalLoss: m.GlobalLoss, TrainingTime: m.TrainingTime, AggregationTime: m.AggregationTime, TotalRoundTime: m.TotalRoundTime, ParameterChange: m.ParameterChange, Contributors: m.Contributors, Rejected: m.Rejected, LocalLossByRound: m.LocalLossByRound, GlobalLossByRound: m.GlobalLossByRound, ParameterChangeByRound: m.ParameterChangeByRound, TrainingTimeByRound: m.TrainingTimeByRound, AggregationTimeByRound: m.AggregationTimeByRound, EffectiveWeights: m.EffectiveWeights, Available: map[string]bool{"live_network_counters": false, "process_cpu": false, "go_heap_memory": true, "trust_metrics": m.AverageTrust > 0}}
}

func scenarioConfig(cfg ExperimentConfig) ExperimentConfig {
	switch cfg.Scenario {
	case "scaling":
		if cfg.Peers == 20 {
			cfg.Peers = 100
		}
	case "highload":
		cfg.DuplicateRate = .3
		cfg.PublishRate *= 2
	case "packetloss":
		cfg.PacketLoss = .05
	case "duplicate":
		cfg.DuplicateRate = .35
	case "churn":
		cfg.ChurnRate = .2
	case "trust":
		cfg.TrustEnabled = true
	case "fl":
		cfg.FLRounds = 5
	}
	return cfg
}

func printSummary(results []Result, cfg ExperimentConfig) {
	fmt.Printf("Trust-Aware FedGreenSub Evaluation\nScenario: %s\nPeers: %d\nMessages: %d\nMessage size: %d bytes\nRepetitions: %d\n\n", cfg.Scenario, cfg.Peers, cfg.Messages, cfg.MessageSize, cfg.Repetitions)
	for _, name := range implementationNames() {
		var delivery, energy, cpu, duplicates float64
		count := 0.0
		for _, result := range results {
			if result.Implementation == name {
				delivery += result.Metrics.DeliveryRatio
				energy += result.Metrics.EnergyPerDeliveredMessage
				cpu += result.Metrics.CPU
				duplicates += result.Metrics.DuplicateRatio
				count++
			}
		}
		if count > 0 {
			fmt.Printf("%-12s delivery=%.4f energy/delivered=%.6f cpu=%.2f duplicate=%.4f\n", name, delivery/count, energy/count, cpu/count, duplicates/count)
		}
	}
}

func printAnalysis(results []Result) {
	summaries := summarize(results)
	if len(summaries) < 2 {
		return
	}
	base := summaries[0]
	fmt.Println("\nAnalysis (positive means lower-is-better reduction for energy/CPU/duplicates)")
	for _, current := range summaries[1:] {
		fmt.Printf("%s vs %s: delivery improvement %+0.2f%%, duplicate reduction %+0.2f%%, energy reduction %+0.2f%%, CPU overhead %+0.2f%%\n", current.Implementation, base.Implementation, percent(current.DeliveryRatio.Mean-base.DeliveryRatio.Mean, base.DeliveryRatio.Mean), percent(base.DuplicateRatio.Mean-current.DuplicateRatio.Mean, base.DuplicateRatio.Mean), percent(base.EnergyPerDeliveredMessage.Mean-current.EnergyPerDeliveredMessage.Mean, base.EnergyPerDeliveredMessage.Mean), percent(current.CPU.Mean-base.CPU.Mean, base.CPU.Mean))
	}
}

func percent(delta, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return delta / denominator * 100
}
