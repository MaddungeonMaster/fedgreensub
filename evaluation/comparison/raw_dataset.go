package main

// raw_dataset.go exports the reproducible, per-run (implementation x
// peer_count x seed) benchmark dataset used for statistical analysis and
// paper figures. It intentionally reuses the existing runExperiments/Result
// pipeline unchanged -- it only reads already-produced Result values and
// writes them out in a different, flatter shape. No evaluator, guardrail, FL,
// or workload behavior is touched here.

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang_project/evaluation/comparison/implementations"
	fedgreensub "golang_project/internal/fedgreensub"
)

// rawDatasetImplementations is the fixed set of three implementations this
// dataset compares, restricted from the full five names runExperiments
// produces (gossipsub, heuristic, fl, fedgreen, trustaware). heuristic and fl
// are intermediate/ablation implementations not part of the validated
// three-way comparison this dataset reports.
func rawDatasetImplementations() []string {
	return []string{"gossipsub", "fedgreen", "trustaware"}
}

// rawDatasetDisplayName maps the internal implementation identifiers used
// throughout runner.go to the display names requested for the dataset.
func rawDatasetDisplayName(name string) string {
	switch name {
	case "gossipsub":
		return "GossipSub"
	case "fedgreen":
		return "FedGreenSub"
	case "trustaware":
		return "Trust-Aware"
	default:
		return name
	}
}

// rawDatasetConfig is the single source of truth for the validated benchmark
// configuration this dataset must use. Seeds are not a separate field:
// Repetitions=5 combined with Seed=4242 reproduces exactly seeds
// 4242..4246, the same mechanism runExperimentsSingle already uses elsewhere
// in this package (workload.RandomSeed += repetition-1).
func rawDatasetConfig() ExperimentConfig {
	return ExperimentConfig{
		Scenario:    "raw-dataset",
		PeerCounts:  []int{10, 25, 50, 100, 200},
		Messages:    200,
		MessageSize: 1024,
		PublishRate: 100,
		Seed:        4242,
		Repetitions: 5,
		FLRounds:    3,
	}.normalize()
}

func rawDatasetSeeds(cfg ExperimentConfig) []int64 {
	seeds := make([]int64, cfg.Repetitions)
	for i := range seeds {
		seeds[i] = cfg.Seed + int64(i)
	}
	return seeds
}

// RawRow is one implementation x peer_count x seed observation.
type RawRow struct {
	Implementation string
	PeerCount      int
	Seed           int64

	Messages         int
	MessageSizeBytes int
	PublishRate      float64
	FLRounds         int

	DeliveryRatio  float64
	DuplicateRatio float64
	LatencyMs      float64

	ResourceCostTotal        float64
	ResourceCostPerDelivered float64

	DeployedMeshDegree   int
	DeployedGossipFactor float64
	DeployedHeartbeatMs  float64

	AcceptedRounds int
	RejectedRounds int

	// Additional existing metrics, already produced by the evaluator,
	// included because they do not require changing the experiment.
	Objective         float64
	EnergyCost        float64 // per-participant normalized modeled resource-cost score (not total, not physical energy)
	DuplicateMessages uint64
	DeliveredMessages uint64
	PublishedMessages uint64
}

func rawRowHeader() []string {
	return []string{
		"implementation", "peer_count", "seed",
		"messages", "message_size_bytes", "publish_rate", "fl_rounds",
		"delivery_ratio", "duplicate_ratio", "latency_ms",
		"resource_cost_total", "resource_cost_per_delivered",
		"deployed_mesh_degree", "deployed_gossip_factor", "deployed_heartbeat_ms",
		"accepted_rounds", "rejected_rounds",
		"objective", "energy_cost", "duplicate_messages", "delivered_messages", "published_messages",
	}
}

func (r RawRow) csvRow() []string {
	return []string{
		r.Implementation, fmt.Sprint(r.PeerCount), fmt.Sprint(r.Seed),
		fmt.Sprint(r.Messages), fmt.Sprint(r.MessageSizeBytes), fmt.Sprintf("%g", r.PublishRate), fmt.Sprint(r.FLRounds),
		fmt.Sprintf("%.10g", r.DeliveryRatio), fmt.Sprintf("%.10g", r.DuplicateRatio), fmt.Sprintf("%.10g", r.LatencyMs),
		fmt.Sprintf("%.10g", r.ResourceCostTotal), fmt.Sprintf("%.10g", r.ResourceCostPerDelivered),
		fmt.Sprint(r.DeployedMeshDegree), fmt.Sprintf("%.10g", r.DeployedGossipFactor), fmt.Sprintf("%.10g", r.DeployedHeartbeatMs),
		fmt.Sprint(r.AcceptedRounds), fmt.Sprint(r.RejectedRounds),
		fmt.Sprintf("%.10g", r.Objective), fmt.Sprintf("%.10g", r.EnergyCost), fmt.Sprint(r.DuplicateMessages), fmt.Sprint(r.DeliveredMessages), fmt.Sprint(r.PublishedMessages),
	}
}

// filterResultsByImplementation keeps only results whose Implementation is in
// names, preserving order. It does not mutate results.
func filterResultsByImplementation(results []Result, names []string) []Result {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	filtered := make([]Result, 0, len(results))
	for _, r := range results {
		if _, ok := wanted[r.Implementation]; ok {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// toRawRow flattens one Result into one RawRow. objectiveCfg supplies the
// same, unmodified MinimumDeliveryRatio/TargetWeights the live evaluator
// uses, so the reported objective is recomputed with fedgreensub.ScoreCandidate
// -- the existing scoring function, not a new formula -- directly from the
// run's own reported outcome. This lets every implementation (including the
// non-adaptive gossipsub baseline, which never produces an FLRoundTrace) get
// a directly comparable objective value.
func toRawRow(r Result, objectiveCfg fedgreensub.Config) RawRow {
	m := r.Metrics
	base := implementations.DefaultParameters()
	deployedGossip, deployedHeartbeatMs := base.GossipFactor, float64(base.HeartbeatInterval.Milliseconds())
	accepted, rejected := 0, 0
	for _, tr := range m.FLRoundTrace {
		if tr.Accepted {
			accepted++
			// The last accepted round's candidate is exactly what run()'s
			// outer parameters variable ends up holding (see
			// evaluation/comparison/implementations/implementation.go
			// run()/runFederatedRounds()), i.e. the actually deployed
			// configuration -- not merely an internally-proposed one.
			deployedGossip = tr.Candidate.GossipFactor
			if hb, err := time.ParseDuration(tr.Candidate.HeartbeatInterval); err == nil {
				deployedHeartbeatMs = float64(hb.Milliseconds())
			}
		} else {
			rejected++
		}
	}

	objective := fedgreensub.ScoreCandidate(fedgreensub.CandidateScore{
		DeliveryRatio:  m.DeliveryRatio,
		DuplicateRatio: m.DuplicateRatio,
		LatencyCost:    m.LatencyAverage / 1000, // LatencyAverage is ms; the evaluator's LatencyCost is normalized (ms/1000)
		EnergyCost:     m.Energy,
		Valid:          true,
	}, objectiveCfg.MinimumDeliveryRatio, objectiveCfg.TargetWeights)

	return RawRow{
		Implementation:   rawDatasetDisplayName(r.Implementation),
		PeerCount:        r.PeerCount,
		Seed:             r.Seed,
		Messages:         r.MessageCount,
		MessageSizeBytes: r.MessageSize,
		PublishRate:      r.PublishRate,
		FLRounds:         int(m.FLRounds),

		DeliveryRatio:  m.DeliveryRatio,
		DuplicateRatio: m.DuplicateRatio,
		LatencyMs:      m.LatencyAverage,

		ResourceCostTotal:        m.ModeledResourceCostTotal,
		ResourceCostPerDelivered: m.ModeledResourceCostPerDeliveredMessage,

		DeployedMeshDegree:   int(m.AverageMeshDegree),
		DeployedGossipFactor: deployedGossip,
		DeployedHeartbeatMs:  deployedHeartbeatMs,

		AcceptedRounds: accepted,
		RejectedRounds: rejected,

		Objective:         objective,
		EnergyCost:        m.Energy,
		DuplicateMessages: m.Duplicates,
		DeliveredMessages: m.Delivered,
		PublishedMessages: m.Published,
	}
}

// generateFilteredResults runs the existing, unmodified benchmark pipeline
// (runExperiments) for the validated configuration and restricts the result
// set to the three implementations this dataset compares. Shared by
// generateRawRows and generateAdaptationTraceRows so both are derived from
// exactly the same underlying run.
func generateFilteredResults() ([]Result, ExperimentConfig, error) {
	cfg := rawDatasetConfig()
	results, err := runExperiments(cfg)
	if err != nil {
		return nil, cfg, err
	}
	return filterResultsByImplementation(results, rawDatasetImplementations()), cfg, nil
}

// generateRawRows runs the existing, unmodified benchmark pipeline
// (runExperiments) for the validated configuration, restricts the result set
// to the three implementations this dataset compares, and flattens each
// individual run into one RawRow. FLRounds in cfg is preserved from
// rawDatasetConfig(); this does not change how many rounds run, only how the
// already-produced results are reported.
func generateRawRows() ([]RawRow, ExperimentConfig, error) {
	filtered, cfg, err := generateFilteredResults()
	if err != nil {
		return nil, cfg, err
	}
	objectiveCfg := fedgreensub.DefaultConfig()
	rows := make([]RawRow, 0, len(filtered))
	for _, r := range filtered {
		rows = append(rows, toRawRow(r, objectiveCfg))
	}
	return rows, cfg, nil
}

func writeRawCSV(path string, rows []RawRow) error {
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
	if err := w.Write(rawRowHeader()); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row.csvRow()); err != nil {
			return err
		}
	}
	return w.Error()
}

// DatasetValidationReport summarizes structural checks on the raw dataset:
// exact row count, distinct dimension sizes, and any gaps or duplicates in
// the implementation x peer_count x seed grid.
type DatasetValidationReport struct {
	TotalRows           int
	Implementations     int
	PeerCounts          int
	Seeds               int
	MissingCombinations []string
	DuplicateRunIDs     []string
}

func runID(implementation string, peerCount int, seed int64) string {
	return fmt.Sprintf("%s|%d|%d", implementation, peerCount, seed)
}

// validateRawDataset checks that rows contains exactly one row per
// (implementation, peer_count, seed) combination implied by cfg and
// rawDatasetImplementations(), with no duplicates and no gaps.
func validateRawDataset(rows []RawRow, cfg ExperimentConfig) DatasetValidationReport {
	seeds := rawDatasetSeeds(cfg)
	implNames := make([]string, 0, len(rawDatasetImplementations()))
	for _, name := range rawDatasetImplementations() {
		implNames = append(implNames, rawDatasetDisplayName(name))
	}

	expected := make(map[string]struct{}, len(implNames)*len(cfg.PeerCounts)*len(seeds))
	for _, impl := range implNames {
		for _, peers := range cfg.PeerCounts {
			for _, seed := range seeds {
				expected[runID(impl, peers, seed)] = struct{}{}
			}
		}
	}

	seen := make(map[string]int, len(rows))
	for _, row := range rows {
		seen[runID(row.Implementation, row.PeerCount, row.Seed)]++
	}

	var missing []string
	for id := range expected {
		if seen[id] == 0 {
			missing = append(missing, id)
		}
	}
	var duplicates []string
	for id, count := range seen {
		if count > 1 {
			duplicates = append(duplicates, fmt.Sprintf("%s (x%d)", id, count))
		}
	}

	return DatasetValidationReport{
		TotalRows:           len(rows),
		Implementations:     len(implNames),
		PeerCounts:          len(cfg.PeerCounts),
		Seeds:               len(seeds),
		MissingCombinations: missing,
		DuplicateRunIDs:     duplicates,
	}
}
