package main

// raw_summary.go aggregates the per-seed RawRow observations produced by
// raw_dataset.go into implementation x peer_count summary statistics. All
// statistics are computed directly from the raw per-seed rows (never from an
// already-averaged table), using sample standard deviation (ddof=1) as
// required for five independent seed observations.

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// SummaryRow aggregates one implementation x peer_count group across seeds.
type SummaryRow struct {
	Implementation string
	PeerCount      int
	Seeds          int

	DeliveryRatio            Stat
	DuplicateRatio           Stat
	LatencyMs                Stat
	ResourceCostTotal        Stat
	ResourceCostPerDelivered Stat

	// Improvement/difference relative to the GossipSub mean at the same
	// peer_count. hasBaseline is false for the GossipSub row itself (and for
	// any peer_count where a GossipSub baseline could not be computed), in
	// which case these fields are left at zero and omitted (blank) in the CSV.
	ResourceCostImprovementPercent   float64
	DuplicateRatioImprovementPercent float64
	LatencyImprovementPercent        float64
	DeliveryRatioDifference          float64
	hasBaseline                      bool
}

// Stat holds sample statistics (ddof=1) for one metric across seeds.
type Stat struct {
	Mean    float64
	StdDev  float64
	Minimum float64
	Maximum float64
	N       int
}

func computeStat(values []float64) Stat {
	n := len(values)
	if n == 0 {
		return Stat{}
	}
	stat := Stat{Minimum: math.Inf(1), Maximum: math.Inf(-1), N: n}
	sum := 0.0
	for _, v := range values {
		sum += v
		if v < stat.Minimum {
			stat.Minimum = v
		}
		if v > stat.Maximum {
			stat.Maximum = v
		}
	}
	stat.Mean = sum / float64(n)
	if n > 1 {
		variance := 0.0
		for _, v := range values {
			d := v - stat.Mean
			variance += d * d
		}
		// Sample standard deviation: ddof = 1 (divide by n-1), not population
		// standard deviation (n).
		stat.StdDev = math.Sqrt(variance / float64(n-1))
	}
	return stat
}

// computeSummary groups rows by (Implementation, PeerCount) and computes
// mean/stddev/min/max for the five headline metrics, plus improvement (or,
// for delivery ratio, plain difference) relative to the GossipSub group at
// the same peer_count.
func computeSummary(rows []RawRow) []SummaryRow {
	type key struct {
		implementation string
		peerCount      int
	}
	groups := make(map[key][]RawRow)
	var order []key
	for _, row := range rows {
		k := key{row.Implementation, row.PeerCount}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], row)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].peerCount != order[j].peerCount {
			return order[i].peerCount < order[j].peerCount
		}
		return order[i].implementation < order[j].implementation
	})

	baselineMeans := make(map[int]SummaryRow) // peer_count -> GossipSub summary (partially filled)
	summaries := make(map[key]SummaryRow, len(order))
	for _, k := range order {
		group := groups[k]
		summary := SummaryRow{Implementation: k.implementation, PeerCount: k.peerCount, Seeds: len(group)}
		summary.DeliveryRatio = computeStat(extract(group, func(r RawRow) float64 { return r.DeliveryRatio }))
		summary.DuplicateRatio = computeStat(extract(group, func(r RawRow) float64 { return r.DuplicateRatio }))
		summary.LatencyMs = computeStat(extract(group, func(r RawRow) float64 { return r.LatencyMs }))
		summary.ResourceCostTotal = computeStat(extract(group, func(r RawRow) float64 { return r.ResourceCostTotal }))
		summary.ResourceCostPerDelivered = computeStat(extract(group, func(r RawRow) float64 { return r.ResourceCostPerDelivered }))
		summaries[k] = summary
		if k.implementation == rawDatasetDisplayName("gossipsub") {
			baselineMeans[k.peerCount] = summary
		}
	}

	result := make([]SummaryRow, 0, len(order))
	for _, k := range order {
		summary := summaries[k]
		if baseline, ok := baselineMeans[k.peerCount]; ok && k.implementation != rawDatasetDisplayName("gossipsub") {
			summary.hasBaseline = true
			summary.ResourceCostImprovementPercent = percentImprovement(baseline.ResourceCostTotal.Mean, summary.ResourceCostTotal.Mean)
			summary.DuplicateRatioImprovementPercent = percentImprovement(baseline.DuplicateRatio.Mean, summary.DuplicateRatio.Mean)
			summary.LatencyImprovementPercent = percentImprovement(baseline.LatencyMs.Mean, summary.LatencyMs.Mean)
			// Delivery ratio: preserve the actual signed difference. A
			// decrease must never be reported as a positive "improvement".
			summary.DeliveryRatioDifference = summary.DeliveryRatio.Mean - baseline.DeliveryRatio.Mean
		}
		result = append(result, summary)
	}
	return result
}

// percentImprovement computes (baseline - value)/baseline*100: positive means
// value is lower (better) than baseline, matching the "improvement_percent"
// convention requested for resource cost, duplicate ratio, and latency
// (all lower-is-better metrics).
func percentImprovement(baseline, value float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (baseline - value) / baseline * 100
}

func extract(rows []RawRow, value func(RawRow) float64) []float64 {
	values := make([]float64, len(rows))
	for i, r := range rows {
		values[i] = value(r)
	}
	return values
}

func summaryRowHeader() []string {
	return []string{
		"implementation", "peer_count", "seeds",
		"delivery_ratio_mean", "delivery_ratio_stddev", "delivery_ratio_min", "delivery_ratio_max",
		"duplicate_ratio_mean", "duplicate_ratio_stddev", "duplicate_ratio_min", "duplicate_ratio_max",
		"latency_ms_mean", "latency_ms_stddev", "latency_ms_min", "latency_ms_max",
		"resource_cost_total_mean", "resource_cost_total_stddev", "resource_cost_total_min", "resource_cost_total_max",
		"resource_cost_per_delivered_mean", "resource_cost_per_delivered_stddev", "resource_cost_per_delivered_min", "resource_cost_per_delivered_max",
		"resource_cost_improvement_percent", "duplicate_ratio_improvement_percent", "latency_improvement_percent", "delivery_ratio_difference",
	}
}

func (s SummaryRow) csvRow() []string {
	f := func(v float64) string { return fmt.Sprintf("%.10g", v) }
	blank := func(v float64) string {
		if !s.hasBaseline {
			return ""
		}
		return f(v)
	}
	return []string{
		s.Implementation, fmt.Sprint(s.PeerCount), fmt.Sprint(s.Seeds),
		f(s.DeliveryRatio.Mean), f(s.DeliveryRatio.StdDev), f(s.DeliveryRatio.Minimum), f(s.DeliveryRatio.Maximum),
		f(s.DuplicateRatio.Mean), f(s.DuplicateRatio.StdDev), f(s.DuplicateRatio.Minimum), f(s.DuplicateRatio.Maximum),
		f(s.LatencyMs.Mean), f(s.LatencyMs.StdDev), f(s.LatencyMs.Minimum), f(s.LatencyMs.Maximum),
		f(s.ResourceCostTotal.Mean), f(s.ResourceCostTotal.StdDev), f(s.ResourceCostTotal.Minimum), f(s.ResourceCostTotal.Maximum),
		f(s.ResourceCostPerDelivered.Mean), f(s.ResourceCostPerDelivered.StdDev), f(s.ResourceCostPerDelivered.Minimum), f(s.ResourceCostPerDelivered.Maximum),
		blank(s.ResourceCostImprovementPercent), blank(s.DuplicateRatioImprovementPercent), blank(s.LatencyImprovementPercent), blank(s.DeliveryRatioDifference),
	}
}

func writeSummaryCSV(path string, rows []SummaryRow) error {
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
	if err := w.Write(summaryRowHeader()); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row.csvRow()); err != nil {
			return err
		}
	}
	return w.Error()
}
