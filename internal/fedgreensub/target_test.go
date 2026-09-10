package fedgreensub

import (
	"testing"
	"time"
)

func TestSelectBestCandidatePicksLowestObjective(t *testing.T) {
	current := GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}
	weights := DefaultTargetWeights()
	candidates := []CandidateScore{
		{Parameters: GossipParameters{MeshDegree: 6, GossipFactor: .20, HeartbeatInterval: 750 * time.Millisecond}, DeliveryRatio: .97, DuplicateRatio: .10, LatencyCost: .05, EnergyCost: .30, Valid: true},
		{Parameters: GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}, DeliveryRatio: .97, DuplicateRatio: .20, LatencyCost: .10, EnergyCost: .40, Valid: true},
		{Parameters: GossipParameters{MeshDegree: 10, GossipFactor: .30, HeartbeatInterval: 1250 * time.Millisecond}, DeliveryRatio: .97, DuplicateRatio: .30, LatencyCost: .15, EnergyCost: .50, Valid: true},
	}
	best, ok := SelectBestCandidate(candidates, .95, weights, current)
	if !ok {
		t.Fatal("expected a candidate to be selected")
	}
	if best.Parameters.MeshDegree != 6 {
		t.Fatalf("expected the genuinely lower-objective candidate (mesh=6), got mesh=%d", best.Parameters.MeshDegree)
	}
}

// TestSelectBestCandidateIsOrderIndependent guards against reintroducing an
// enumeration-order bias: reversing (or shuffling) the input slice must not
// change which candidate is selected, whether candidates have genuinely
// different objectives or are exactly tied.
func TestSelectBestCandidateIsOrderIndependent(t *testing.T) {
	current := GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}
	weights := DefaultTargetWeights()

	distinct := []CandidateScore{
		{Parameters: GossipParameters{MeshDegree: 6, GossipFactor: .20, HeartbeatInterval: 750 * time.Millisecond}, DeliveryRatio: .97, DuplicateRatio: .10, LatencyCost: .05, EnergyCost: .30, Valid: true},
		{Parameters: GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}, DeliveryRatio: .97, DuplicateRatio: .20, LatencyCost: .10, EnergyCost: .40, Valid: true},
		{Parameters: GossipParameters{MeshDegree: 10, GossipFactor: .30, HeartbeatInterval: 1250 * time.Millisecond}, DeliveryRatio: .97, DuplicateRatio: .30, LatencyCost: .15, EnergyCost: .50, Valid: true},
	}
	forward, ok := SelectBestCandidate(distinct, .95, weights, current)
	if !ok {
		t.Fatal("expected a candidate to be selected")
	}
	reversed := []CandidateScore{distinct[2], distinct[1], distinct[0]}
	backward, ok := SelectBestCandidate(reversed, .95, weights, current)
	if !ok {
		t.Fatal("expected a candidate to be selected")
	}
	if forward.Parameters != backward.Parameters {
		t.Fatalf("candidate selection depended on enumeration order: forward=%+v backward=%+v", forward.Parameters, backward.Parameters)
	}

	// Exact objective tie: mesh 6 and mesh 8 produce identical delivery,
	// duplicate, latency, and energy figures. Without an explicit tie-break,
	// the winner would silently depend on slice order.
	tied := []CandidateScore{
		{Parameters: GossipParameters{MeshDegree: 6, GossipFactor: .25, HeartbeatInterval: time.Second}, DeliveryRatio: .97, DuplicateRatio: .10, LatencyCost: .05, EnergyCost: .30, Valid: true},
		{Parameters: GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}, DeliveryRatio: .97, DuplicateRatio: .10, LatencyCost: .05, EnergyCost: .30, Valid: true},
	}
	tiedForward, ok := SelectBestCandidate(tied, .95, weights, current)
	if !ok {
		t.Fatal("expected a candidate to be selected")
	}
	tiedBackward, ok := SelectBestCandidate([]CandidateScore{tied[1], tied[0]}, .95, weights, current)
	if !ok {
		t.Fatal("expected a candidate to be selected")
	}
	if tiedForward.Parameters != tiedBackward.Parameters {
		t.Fatalf("tied candidate selection depended on enumeration order: forward=%+v backward=%+v", tiedForward.Parameters, tiedBackward.Parameters)
	}
	// The documented tie-break (smaller change from the deployed
	// configuration) must have actually decided the tie, not enumeration
	// order: mesh=6 is farther from current=8 than mesh=8 is from itself, so
	// mesh=8 (zero change) must win.
	if tiedForward.Parameters.MeshDegree != 8 {
		t.Fatalf("expected the tie to be broken toward the smaller change from current (mesh=8), got mesh=%d", tiedForward.Parameters.MeshDegree)
	}
}

func TestCandidateLessTieBreakPriority(t *testing.T) {
	current := GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}

	// 1. Objective outside tolerance always wins, regardless of the other
	// criteria.
	lowerObjective := CandidateScore{Parameters: GossipParameters{MeshDegree: 10, GossipFactor: .30, HeartbeatInterval: 1250 * time.Millisecond}, Objective: .1, EnergyCost: .9}
	higherObjective := CandidateScore{Parameters: GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}, Objective: .2, EnergyCost: .1}
	if !candidateLess(lowerObjective, higherObjective, current) {
		t.Fatal("expected the lower-objective candidate to win despite a larger parameter change and higher resource cost")
	}

	// 2. Equal objective: smaller change from current wins.
	closeToCurrent := CandidateScore{Parameters: GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}, Objective: .2, EnergyCost: .5}
	farFromCurrent := CandidateScore{Parameters: GossipParameters{MeshDegree: 10, GossipFactor: .30, HeartbeatInterval: 1250 * time.Millisecond}, Objective: .2, EnergyCost: .1}
	if !candidateLess(closeToCurrent, farFromCurrent, current) {
		t.Fatal("expected the candidate closer to the deployed configuration to win an objective tie, even with a higher resource cost")
	}

	// 3. Equal objective and equal distance from current: lower resource
	// cost wins.
	current2 := GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}
	cheaper := CandidateScore{Parameters: GossipParameters{MeshDegree: 6, GossipFactor: .25, HeartbeatInterval: time.Second}, Objective: .2, EnergyCost: .1}
	pricier := CandidateScore{Parameters: GossipParameters{MeshDegree: 10, GossipFactor: .25, HeartbeatInterval: time.Second}, Objective: .2, EnergyCost: .2}
	if !candidateLess(cheaper, pricier, current2) {
		t.Fatal("expected the lower modeled-resource-cost candidate to win after objective and distance-from-current ties")
	}

	// 4. Fully tied: deterministic parameter ordering (mesh, then gossip,
	// then heartbeat) decides.
	a := CandidateScore{Parameters: GossipParameters{MeshDegree: 6, GossipFactor: .25, HeartbeatInterval: time.Second}, Objective: .2, EnergyCost: .3}
	b := CandidateScore{Parameters: GossipParameters{MeshDegree: 10, GossipFactor: .25, HeartbeatInterval: time.Second}, Objective: .2, EnergyCost: .3}
	// Distances from current2 (mesh 8) are both 1 unit of meshSpan (2), so
	// this compares equal on criterion 2 as well; resource cost is tied too,
	// so criterion 4 must decide, and lower mesh degree wins.
	if !candidateLess(a, b, current2) {
		t.Fatal("expected the deterministic parameter ordering (lower mesh degree) to decide a fully tied comparison")
	}
}

func TestParametersTargetUsesCurrentAsReference(t *testing.T) {
	cfg := DefaultConfig()
	currentA := GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}
	currentB := GossipParameters{MeshDegree: 6, GossipFactor: .25, HeartbeatInterval: time.Second}
	candidate := GossipParameters{MeshDegree: 6, GossipFactor: .25, HeartbeatInterval: time.Second}

	targetFromA := ParametersTarget(candidate, cfg, currentA)
	targetFromB := ParametersTarget(candidate, cfg, currentB)
	if targetFromA[0] == targetFromB[0] {
		t.Fatalf("expected the same absolute mesh candidate to encode a different relative target depending on the deployed reference: fromA=%v fromB=%v", targetFromA[0], targetFromB[0])
	}
	// mesh=6 relative to a deployed mesh of 6 must be the "no change" midpoint.
	if targetFromB[0] != .5 {
		t.Fatalf("expected mesh target relative to an identical deployed mesh to be 0.5 (no change), got %v", targetFromB[0])
	}
}
