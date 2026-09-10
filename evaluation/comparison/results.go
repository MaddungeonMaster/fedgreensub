package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

type Result struct {
	ExperimentID   string            `json:"experiment_id"`
	Timestamp      time.Time         `json:"timestamp"`
	Scenario       string            `json:"scenario"`
	Implementation string            `json:"implementation"`
	PeerCount      int               `json:"peer_count"`
	MessageCount   int               `json:"message_count"`
	MessageSize    int               `json:"message_size"`
	PublishRate    float64           `json:"publish_rate"`
	Duration       string            `json:"duration"`
	Seed           int64             `json:"seed"`
	Repetition     int               `json:"repetition"`
	Metrics        ExperimentMetrics `json:"metrics"`
}

type MetricSummary struct {
	Mean    float64 `json:"mean"`
	StdDev  float64 `json:"stddev"`
	Minimum float64 `json:"minimum"`
	Maximum float64 `json:"maximum"`
}
type ImplementationSummary struct {
	Scenario                  string        `json:"scenario"`
	Implementation            string        `json:"implementation"`
	DeliveryRatio             MetricSummary `json:"delivery_ratio"`
	DuplicateRatio            MetricSummary `json:"duplicate_ratio"`
	EnergyPerDeliveredMessage MetricSummary `json:"energy_per_delivered_message"`
	CPU                       MetricSummary `json:"cpu"`
	MemoryMB                  MetricSummary `json:"memory_mb"`
	LatencyAverage            MetricSummary `json:"latency_average"`
	ParameterChange           MetricSummary `json:"parameter_change"`
	GlobalLoss                MetricSummary `json:"global_loss"`
}

func summarize(results []Result) []ImplementationSummary {
	var summaries []ImplementationSummary
	for _, name := range []string{"gossipsub", "heuristic", "fl", "fedgreen", "trustaware"} {
		values := make([]Result, 0)
		for _, result := range results {
			if result.Implementation == name {
				values = append(values, result)
			}
		}
		if len(values) == 0 {
			continue
		}
		summary := ImplementationSummary{Scenario: values[0].Scenario, Implementation: name}
		summary.DeliveryRatio = stats(values, func(r Result) float64 { return r.Metrics.DeliveryRatio })
		summary.DuplicateRatio = stats(values, func(r Result) float64 { return r.Metrics.DuplicateRatio })
		summary.EnergyPerDeliveredMessage = stats(values, func(r Result) float64 { return r.Metrics.EnergyPerDeliveredMessage })
		summary.CPU = stats(values, func(r Result) float64 { return r.Metrics.CPU })
		summary.MemoryMB = stats(values, func(r Result) float64 { return r.Metrics.MemoryMB })
		summary.LatencyAverage = stats(values, func(r Result) float64 { return r.Metrics.LatencyAverage })
		summary.ParameterChange = stats(values, func(r Result) float64 { return r.Metrics.ParameterChange })
		summary.GlobalLoss = stats(values, func(r Result) float64 { return r.Metrics.GlobalLoss })
		summaries = append(summaries, summary)
	}
	return summaries
}

func stats(results []Result, value func(Result) float64) MetricSummary {
	out := MetricSummary{Minimum: math.Inf(1), Maximum: math.Inf(-1)}
	if len(results) == 0 {
		return MetricSummary{}
	}
	for _, result := range results {
		v := value(result)
		out.Mean += v
		if v < out.Minimum {
			out.Minimum = v
		}
		if v > out.Maximum {
			out.Maximum = v
		}
	}
	out.Mean /= float64(len(results))
	for _, result := range results {
		d := value(result) - out.Mean
		out.StdDev += d * d
	}
	out.StdDev = math.Sqrt(out.StdDev / float64(len(results)))
	return out
}

func saveResult(root string, result Result) (string, error) {
	dir := filepath.Join(root, result.Scenario)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%d-%s.json", result.Implementation, result.PeerCount, result.ExperimentID))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0644)
}

func writeCSV(path string, results []Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"scenario", "implementation", "peers", "repetition", "cpu", "memory", "bytes_sent", "bytes_received", "delivery_ratio", "duplicate_ratio", "latency_avg", "latency_p95", "mesh_degree", "energy", "energy_per_delivered_message", "modeled_resource_cost_total", "modeled_resource_cost_per_delivered_message", "average_trust", "minimum_trust", "trust_variance", "trusted_peer_ratio", "trust_update_time", "peer_ranking_time", "fl_rounds", "training_loss", "global_loss", "training_time_ms", "aggregation_time_ms", "total_round_time_ms", "parameter_change", "contributors", "rejected", "effective_weights"}); err != nil {
		return err
	}
	for _, r := range results {
		m := r.Metrics
		weights, _ := json.Marshal(m.EffectiveWeights)
		row := []string{r.Scenario, r.Implementation, fmt.Sprint(r.PeerCount), fmt.Sprint(r.Repetition), fmt.Sprintf("%g", m.CPU), fmt.Sprintf("%g", m.MemoryMB), fmt.Sprint(m.BytesSent), fmt.Sprint(m.BytesReceived), fmt.Sprintf("%g", m.DeliveryRatio), fmt.Sprintf("%g", m.DuplicateRatio), fmt.Sprintf("%g", m.LatencyAverage), fmt.Sprintf("%g", m.LatencyP95), fmt.Sprintf("%g", m.AverageMeshDegree), fmt.Sprintf("%g", m.Energy), fmt.Sprintf("%g", m.EnergyPerDeliveredMessage), fmt.Sprintf("%g", m.ModeledResourceCostTotal), fmt.Sprintf("%g", m.ModeledResourceCostPerDeliveredMessage), fmt.Sprintf("%g", m.AverageTrust), fmt.Sprintf("%g", m.MinimumTrust), fmt.Sprintf("%g", m.TrustVariance), fmt.Sprintf("%g", m.TrustedPeerRatio), fmt.Sprintf("%g", m.TrustUpdateTime), fmt.Sprintf("%g", m.PeerRankingTime), fmt.Sprint(m.FLRounds), fmt.Sprintf("%g", m.TrainingLoss), fmt.Sprintf("%g", m.GlobalLoss), fmt.Sprintf("%g", m.TrainingTime), fmt.Sprintf("%g", m.AggregationTime), fmt.Sprintf("%g", m.TotalRoundTime), fmt.Sprintf("%g", m.ParameterChange), fmt.Sprint(m.Contributors), fmt.Sprint(m.Rejected), string(weights)}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}
