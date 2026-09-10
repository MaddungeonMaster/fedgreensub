package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// EnergyRawRecord is the machine-readable per-run record for the multi-size
// controlled energy experiment. Cost fields are modeled resource-cost units,
// never Joules.
type EnergyRawRecord struct {
	Scenario                               string  `json:"scenario"`
	Implementation                         string  `json:"implementation"`
	PeerCount                              int     `json:"peer_count"`
	Repetition                             int     `json:"repetition"`
	Seed                                   int64   `json:"seed"`
	ModeledResourceCostTotal               float64 `json:"modeled_resource_cost_total"`
	ModeledResourceCostPerDeliveredMessage float64 `json:"modeled_resource_cost_per_delivered_message"`
	Delivered                              uint64  `json:"delivered"`
	Duplicates                             uint64  `json:"duplicates"`
	DuplicateRatio                         float64 `json:"duplicate_ratio"`
	DeliveryRatio                          float64 `json:"delivery_ratio"`
	BytesSent                              uint64  `json:"bytes_sent"`
	BytesReceived                          uint64  `json:"bytes_received"`
	CPU                                    float64 `json:"cpu"`
	MemoryMB                               float64 `json:"memory_mb"`
}

type EnergySummaryRecord struct {
	PeerCount                             int     `json:"peer_count"`
	Implementation                        string  `json:"implementation"`
	Repetitions                           int     `json:"repetitions"`
	ModeledResourceCostTotalMean          float64 `json:"modeled_resource_cost_total_mean"`
	ModeledResourceCostTotalStdDev        float64 `json:"modeled_resource_cost_total_stddev"`
	ModeledResourceCostPerDeliveredMean   float64 `json:"modeled_resource_cost_per_delivered_mean"`
	ModeledResourceCostPerDeliveredStdDev float64 `json:"modeled_resource_cost_per_delivered_stddev"`
	DeliveredMean                         float64 `json:"delivered_mean"`
	DuplicatesMean                        float64 `json:"duplicates_mean"`
	DuplicateRatioMean                    float64 `json:"duplicate_ratio_mean"`
	DeliveryRatioMean                     float64 `json:"delivery_ratio_mean"`
	BytesSentMean                         float64 `json:"bytes_sent_mean"`
	BytesReceivedMean                     float64 `json:"bytes_received_mean"`
	CPUMean                               float64 `json:"cpu_mean"`
	MemoryMBMean                          float64 `json:"memory_mb_mean"`
	BaselineTotalMean                     float64 `json:"gossipsub_baseline_total_mean"`
	AbsoluteDifferenceFromBaseline        float64 `json:"absolute_difference_from_baseline"`
	ReductionFromBaselinePercent          float64 `json:"reduction_from_baseline_percent"`
	TrustAwareVsFedGreenDifference        float64 `json:"trustaware_vs_fedgreen_difference"`
	TrustAwareVsFedGreenReductionPercent  float64 `json:"trustaware_vs_fedgreen_reduction_percent"`
}

func writeEnergyScalingReport(dir string, results []Result) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	raw := make([]EnergyRawRecord, 0, len(results))
	for _, result := range results {
		m := result.Metrics
		raw = append(raw, EnergyRawRecord{Scenario: result.Scenario, Implementation: result.Implementation, PeerCount: result.PeerCount, Repetition: result.Repetition, Seed: result.Seed, ModeledResourceCostTotal: m.ModeledResourceCostTotal, ModeledResourceCostPerDeliveredMessage: m.ModeledResourceCostPerDeliveredMessage, Delivered: m.Delivered, Duplicates: m.Duplicates, DuplicateRatio: m.DuplicateRatio, DeliveryRatio: m.DeliveryRatio, BytesSent: m.BytesSent, BytesReceived: m.BytesReceived, CPU: m.CPU, MemoryMB: m.MemoryMB})
	}
	jsonBytes, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "energy_scaling_raw.json"), jsonBytes, 0644); err != nil {
		return err
	}
	if err := writeEnergyRawCSV(filepath.Join(dir, "energy_scaling_raw.csv"), raw); err != nil {
		return err
	}
	summary := summarizeEnergyScaling(raw)
	summaryBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "energy_scaling_summary.json"), summaryBytes, 0644); err != nil {
		return err
	}
	return writeEnergySummaryCSV(filepath.Join(dir, "energy_scaling_summary.csv"), summary)
}

func writeEnergyRawCSV(path string, records []EnergyRawRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"scenario", "implementation", "peer_count", "repetition", "seed", "modeled_resource_cost_total", "modeled_resource_cost_per_delivered_message", "delivered", "duplicates", "duplicate_ratio", "delivery_ratio", "bytes_sent", "bytes_received", "cpu", "memory_mb"}); err != nil {
		return err
	}
	for _, r := range records {
		if err := w.Write([]string{r.Scenario, r.Implementation, strconv.Itoa(r.PeerCount), strconv.Itoa(r.Repetition), strconv.FormatInt(r.Seed, 10), fmt.Sprintf("%.12g", r.ModeledResourceCostTotal), fmt.Sprintf("%.12g", r.ModeledResourceCostPerDeliveredMessage), strconv.FormatUint(r.Delivered, 10), strconv.FormatUint(r.Duplicates, 10), fmt.Sprintf("%.12g", r.DuplicateRatio), fmt.Sprintf("%.12g", r.DeliveryRatio), strconv.FormatUint(r.BytesSent, 10), strconv.FormatUint(r.BytesReceived, 10), fmt.Sprintf("%.12g", r.CPU), fmt.Sprintf("%.12g", r.MemoryMB)}); err != nil {
			return err
		}
	}
	return w.Error()
}

func summarizeEnergyScaling(raw []EnergyRawRecord) []EnergySummaryRecord {
	groups := make(map[int]map[string][]EnergyRawRecord)
	for _, record := range raw {
		if groups[record.PeerCount] == nil {
			groups[record.PeerCount] = make(map[string][]EnergyRawRecord)
		}
		groups[record.PeerCount][record.Implementation] = append(groups[record.PeerCount][record.Implementation], record)
	}
	peers := make([]int, 0, len(groups))
	for peerCount := range groups {
		peers = append(peers, peerCount)
	}
	sort.Ints(peers)
	summary := make([]EnergySummaryRecord, 0, len(raw))
	for _, peerCount := range peers {
		byImplementation := groups[peerCount]
		baseline := meanRaw(byImplementation["gossipsub"], func(r EnergyRawRecord) float64 { return r.ModeledResourceCostTotal })
		fedgreen := meanRaw(byImplementation["fedgreen"], func(r EnergyRawRecord) float64 { return r.ModeledResourceCostTotal })
		trust := meanRaw(byImplementation["trustaware"], func(r EnergyRawRecord) float64 { return r.ModeledResourceCostTotal })
		for _, implementation := range []string{"gossipsub", "fedgreen", "trustaware"} {
			runs := byImplementation[implementation]
			if len(runs) == 0 {
				continue
			}
			totalMean, totalStd := meanStdRaw(runs, func(r EnergyRawRecord) float64 { return r.ModeledResourceCostTotal })
			perDeliveredMean, perDeliveredStd := meanStdRaw(runs, func(r EnergyRawRecord) float64 { return r.ModeledResourceCostPerDeliveredMessage })
			difference := totalMean - baseline
			reduction := 0.0
			if baseline != 0 {
				reduction = -difference / baseline * 100
			}
			trustDifference := trust - fedgreen
			trustReduction := 0.0
			if fedgreen != 0 {
				trustReduction = -trustDifference / fedgreen * 100
			}
			summary = append(summary, EnergySummaryRecord{PeerCount: peerCount, Implementation: implementation, Repetitions: len(runs), ModeledResourceCostTotalMean: totalMean, ModeledResourceCostTotalStdDev: totalStd, ModeledResourceCostPerDeliveredMean: perDeliveredMean, ModeledResourceCostPerDeliveredStdDev: perDeliveredStd, DeliveredMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return float64(r.Delivered) }), DuplicatesMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return float64(r.Duplicates) }), DuplicateRatioMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return r.DuplicateRatio }), DeliveryRatioMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return r.DeliveryRatio }), BytesSentMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return float64(r.BytesSent) }), BytesReceivedMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return float64(r.BytesReceived) }), CPUMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return r.CPU }), MemoryMBMean: meanRaw(runs, func(r EnergyRawRecord) float64 { return r.MemoryMB }), BaselineTotalMean: baseline, AbsoluteDifferenceFromBaseline: difference, ReductionFromBaselinePercent: reduction, TrustAwareVsFedGreenDifference: trustDifference, TrustAwareVsFedGreenReductionPercent: trustReduction})
		}
	}
	return summary
}

func meanRaw(records []EnergyRawRecord, value func(EnergyRawRecord) float64) float64 {
	if len(records) == 0 {
		return 0
	}
	total := 0.0
	for _, record := range records {
		total += value(record)
	}
	return total / float64(len(records))
}
func meanStdRaw(records []EnergyRawRecord, value func(EnergyRawRecord) float64) (float64, float64) {
	mean := meanRaw(records, value)
	if len(records) < 2 {
		return mean, 0
	}
	sum := 0.0
	for _, record := range records {
		delta := value(record) - mean
		sum += delta * delta
	}
	return mean, math.Sqrt(sum / float64(len(records)-1))
}

func writeEnergySummaryCSV(path string, records []EnergySummaryRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"peer_count", "implementation", "repetitions", "modeled_resource_cost_total_mean", "modeled_resource_cost_total_stddev", "modeled_resource_cost_per_delivered_mean", "modeled_resource_cost_per_delivered_stddev", "delivered_mean", "duplicates_mean", "duplicate_ratio_mean", "delivery_ratio_mean", "bytes_sent_mean", "bytes_received_mean", "cpu_mean", "memory_mb_mean", "gossipsub_baseline_total_mean", "absolute_difference_from_baseline", "reduction_from_baseline_percent", "trustaware_vs_fedgreen_difference", "trustaware_vs_fedgreen_reduction_percent"}); err != nil {
		return err
	}
	for _, r := range records {
		row := []string{strconv.Itoa(r.PeerCount), r.Implementation, strconv.Itoa(r.Repetitions), fmt.Sprintf("%.12g", r.ModeledResourceCostTotalMean), fmt.Sprintf("%.12g", r.ModeledResourceCostTotalStdDev), fmt.Sprintf("%.12g", r.ModeledResourceCostPerDeliveredMean), fmt.Sprintf("%.12g", r.ModeledResourceCostPerDeliveredStdDev), fmt.Sprintf("%.12g", r.DeliveredMean), fmt.Sprintf("%.12g", r.DuplicatesMean), fmt.Sprintf("%.12g", r.DuplicateRatioMean), fmt.Sprintf("%.12g", r.DeliveryRatioMean), fmt.Sprintf("%.12g", r.BytesSentMean), fmt.Sprintf("%.12g", r.BytesReceivedMean), fmt.Sprintf("%.12g", r.CPUMean), fmt.Sprintf("%.12g", r.MemoryMBMean), fmt.Sprintf("%.12g", r.BaselineTotalMean), fmt.Sprintf("%.12g", r.AbsoluteDifferenceFromBaseline), fmt.Sprintf("%.12g", r.ReductionFromBaselinePercent), fmt.Sprintf("%.12g", r.TrustAwareVsFedGreenDifference), fmt.Sprintf("%.12g", r.TrustAwareVsFedGreenReductionPercent)}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}
