package main

// raw_trace.go exports the existing, already-produced per-round FL adaptation
// trace (Result.Metrics.FLRoundTrace) to a flat CSV, read-only. It reads data
// the benchmark pipeline already generates for every FedGreenSub/Trust-Aware
// run; it does not add any new instrumentation to the FL loop, guardrail, or
// evaluator, and GossipSub (which never runs FL rounds) simply contributes no
// rows, matching what the data actually contains.

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AdaptationTraceRow is one FL round's guardrail decision for one
// implementation x peer_count x seed run.
type AdaptationTraceRow struct {
	PeerCount      int
	Seed           int64
	Implementation string
	Round          uint64

	PreviousMesh  int
	CandidateMesh int
	DeployedMesh  int

	// GossipFactor/HeartbeatMs are the DEPLOYED (post-guardrail-decision)
	// values for this round: the round's candidate values if accepted,
	// otherwise unchanged from the previous round -- mirroring DeployedMesh,
	// so the trajectory reflects what was actually running, not merely what
	// was proposed.
	GossipFactor float64
	HeartbeatMs  float64

	Accepted        bool
	RejectionReason string
}

func adaptationTraceHeader() []string {
	return []string{
		"peer_count", "seed", "implementation", "round",
		"previous_mesh", "candidate_mesh", "deployed_mesh",
		"gossip_factor", "heartbeat_ms",
		"accepted", "rejection_reason",
	}
}

func (r AdaptationTraceRow) csvRow() []string {
	return []string{
		fmt.Sprint(r.PeerCount), fmt.Sprint(r.Seed), r.Implementation, fmt.Sprint(r.Round),
		fmt.Sprint(r.PreviousMesh), fmt.Sprint(r.CandidateMesh), fmt.Sprint(r.DeployedMesh),
		fmt.Sprintf("%.10g", r.GossipFactor), fmt.Sprintf("%.10g", r.HeartbeatMs),
		fmt.Sprint(r.Accepted), r.RejectionReason,
	}
}

// toAdaptationTraceRows flattens every round of every result's FLRoundTrace
// into one row each. results is expected to already be filtered to the
// dataset's three implementations (see filterResultsByImplementation);
// implementations with no FL rounds (GossipSub) simply contribute no rows.
func toAdaptationTraceRows(results []Result) []AdaptationTraceRow {
	var rows []AdaptationTraceRow
	for _, r := range results {
		for _, tr := range r.Metrics.FLRoundTrace {
			gossip, heartbeatMs := tr.Current.GossipFactor, 0.0
			if hb, err := time.ParseDuration(tr.Current.HeartbeatInterval); err == nil {
				heartbeatMs = float64(hb.Milliseconds())
			}
			deployedMesh := tr.Current.MeshDegree
			if tr.Accepted {
				deployedMesh = tr.Candidate.MeshDegree
				gossip = tr.Candidate.GossipFactor
				if hb, err := time.ParseDuration(tr.Candidate.HeartbeatInterval); err == nil {
					heartbeatMs = float64(hb.Milliseconds())
				}
			}
			rows = append(rows, AdaptationTraceRow{
				PeerCount:       r.PeerCount,
				Seed:            r.Seed,
				Implementation:  rawDatasetDisplayName(r.Implementation),
				Round:           tr.Round,
				PreviousMesh:    tr.Current.MeshDegree,
				CandidateMesh:   tr.Candidate.MeshDegree,
				DeployedMesh:    deployedMesh,
				GossipFactor:    gossip,
				HeartbeatMs:     heartbeatMs,
				Accepted:        tr.Accepted,
				RejectionReason: tr.RejectionReason,
			})
		}
	}
	return rows
}

// generateAdaptationTraceRows reuses the same filtered result set
// generateRawRows is built from (same benchmark run, same configuration) and
// flattens every FL round in it.
func generateAdaptationTraceRows() ([]AdaptationTraceRow, error) {
	filtered, _, err := generateFilteredResults()
	if err != nil {
		return nil, err
	}
	return toAdaptationTraceRows(filtered), nil
}

func writeAdaptationTraceCSV(path string, rows []AdaptationTraceRow) error {
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
	if err := w.Write(adaptationTraceHeader()); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row.csvRow()); err != nil {
			return err
		}
	}
	return w.Error()
}
