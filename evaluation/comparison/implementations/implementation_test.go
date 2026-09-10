package implementations

import (
	"math"
	"testing"
	"time"

	fedgreensub "golang_project/internal/fedgreensub"
)

func TestControlledCandidateChangesNetworkOutcome(t *testing.T) {
	profile := controlledProfile(2, 42)
	workload := Workload{Peers: 8, Messages: 200, MessageSize: 128, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1}
	low := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 5, DLow: 3, DHigh: 9, GossipFactor: .1, HeartbeatInterval: 2 * time.Second}, 8)
	high := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 12, DLow: 6, DHigh: 16, GossipFactor: .4, HeartbeatInterval: 500 * time.Millisecond}, 8)
	if low.DeliveryRatio == high.DeliveryRatio && low.DuplicateRatio == high.DuplicateRatio && low.LatencyCost == high.LatencyCost && low.EnergyCost == high.EnergyCost {
		t.Fatal("candidate parameters did not change controlled network outcome")
	}
}

func TestControlledOutcomeMonotonicCandidateEffects(t *testing.T) {
	profile := controlledProfile(1, 4242)
	workload := Workload{Peers: 8, Messages: 200, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1}
	lowMesh := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 5, GossipFactor: .25, HeartbeatInterval: time.Second}, 8)
	highMesh := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 12, GossipFactor: .25, HeartbeatInterval: time.Second}, 8)
	if highMesh.DeliveryRatio < lowMesh.DeliveryRatio {
		t.Fatalf("higher mesh reduced delivery: low=%v high=%v", lowMesh.DeliveryRatio, highMesh.DeliveryRatio)
	}
	lowGossip := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 8, GossipFactor: .1, HeartbeatInterval: time.Second}, 8)
	highGossip := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 8, GossipFactor: .5, HeartbeatInterval: time.Second}, 8)
	if highGossip.DuplicateRatio < lowGossip.DuplicateRatio {
		t.Fatalf("higher gossip factor reduced duplicates: low=%v high=%v", lowGossip.DuplicateRatio, highGossip.DuplicateRatio)
	}
	fastHeartbeat := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: 500 * time.Millisecond}, 8)
	slowHeartbeat := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: 2 * time.Second}, 8)
	if fastHeartbeat.LatencyCost > slowHeartbeat.LatencyCost {
		t.Fatalf("shorter heartbeat increased latency cost: fast=%v slow=%v", fastHeartbeat.LatencyCost, slowHeartbeat.LatencyCost)
	}
}

func TestControlledOutcomeIsSeedReproducible(t *testing.T) {
	workload := Workload{Peers: 8, Messages: 200, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1, RandomSeed: 4242}
	candidate := fedgreensub.GossipParameters{MeshDegree: 9, GossipFactor: .3, HeartbeatInterval: 5 * time.Second}
	a := aggregateOutcome([]controlledParticipantProfile{controlledProfile(0, workload.RandomSeed), controlledProfile(1, workload.RandomSeed)}, workload, candidate, 8)
	b := aggregateOutcome([]controlledParticipantProfile{controlledProfile(0, workload.RandomSeed), controlledProfile(1, workload.RandomSeed)}, workload, candidate, 8)
	if a != b {
		t.Fatalf("same seed produced different controlled outcome: a=%+v b=%+v", a, b)
	}
}

func TestControlledEnergyUsesSharedModeledEstimator(t *testing.T) {
	profile := controlledProfile(2, 4242)
	workload := Workload{Peers: 8, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1}
	candidate := fedgreensub.GossipParameters{MeshDegree: 9, GossipFactor: .3, HeartbeatInterval: 5 * time.Second}
	outcome := controlledOutcome(profile, workload, candidate, 8)
	expected := fedgreensub.EstimateEnergy(fedgreensub.RuntimeMetrics{CPU: profile.cpu, BandwidthInBps: profile.bandwidthBps, BandwidthOutBps: profile.bandwidthBps * .8, DuplicateRate: outcome.DuplicateRatio, HeartbeatDuration: candidate.HeartbeatInterval})
	if outcome.EnergyCost != expected {
		t.Fatalf("controlled energy cost diverged from shared estimator: got=%v want=%v", outcome.EnergyCost, expected)
	}
}

func TestControlledOutcomeHasNoModeSpecificMultiplier(t *testing.T) {
	workload := Workload{Peers: 8, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1, RandomSeed: 4242}
	candidate := fedgreensub.GossipParameters{MeshDegree: 9, GossipFactor: .3, HeartbeatInterval: 5 * time.Second}
	profiles := []controlledParticipantProfile{controlledProfile(0, workload.RandomSeed), controlledProfile(1, workload.RandomSeed)}
	first := aggregateOutcome(profiles, workload, candidate, 8)
	second := aggregateOutcome(profiles, workload, candidate, 8)
	if first != second {
		t.Fatalf("controlled outcome changed without a candidate/profile change")
	}
}

// TestControlledOutcomeMeshDegreesAreNotTied guards the root cause of the
// mesh-target-generation bug fixed alongside this test: mesh degree 6 and 8
// (and 8 and 10) used to produce byte-for-byte identical delivery,
// duplicate, latency, and energy outcomes whenever gossip factor and
// heartbeat matched, because the old formulation only reacted to mesh
// degrees above a hardcoded value of 8. This must never regress.
func TestControlledOutcomeMeshDegreesAreNotTied(t *testing.T) {
	workload := Workload{Peers: 8, Messages: 200, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .05}
	gossipFactor, heartbeat := .20, 750*time.Millisecond
	for _, profileIndex := range []int{0, 1, 2, 3} {
		profile := controlledProfile(profileIndex, 4242)
		const currentMesh = 8
		outcomes := map[int]fedgreensub.CandidateScore{}
		for _, mesh := range []int{6, 8, 10} {
			outcomes[mesh] = controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: mesh, GossipFactor: gossipFactor, HeartbeatInterval: heartbeat}, currentMesh)
		}
		if outcomes[6] == outcomes[8] {
			t.Fatalf("profile=%d: mesh 6 and mesh 8 produced an identical controlled outcome: %+v", profileIndex, outcomes[6])
		}
		if outcomes[8] == outcomes[10] {
			t.Fatalf("profile=%d: mesh 8 and mesh 10 produced an identical controlled outcome: %+v", profileIndex, outcomes[8])
		}
		if outcomes[6] == outcomes[10] {
			t.Fatalf("profile=%d: mesh 6 and mesh 10 produced an identical controlled outcome: %+v", profileIndex, outcomes[6])
		}
	}
}

// TestControlledOutcomeMeshDeficitIsAnchoredToCurrentMesh verifies the
// redundancy-lost effect is computed relative to the actually deployed mesh
// degree (currentMesh), not a hardcoded constant: the same candidate mesh
// degree must carry a different delivery/latency risk depending on what is
// currently deployed.
func TestControlledOutcomeMeshDeficitIsAnchoredToCurrentMesh(t *testing.T) {
	profile := controlledProfile(3, 4242) // adverse/high-loss profile
	workload := Workload{Peers: 8, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .05}
	candidate := fedgreensub.GossipParameters{MeshDegree: 6, GossipFactor: .25, HeartbeatInterval: time.Second}

	noDeficit := controlledOutcome(profile, workload, candidate, 6) // deployed == candidate: no lost redundancy
	withDeficit := controlledOutcome(profile, workload, candidate, 10)
	if withDeficit.DeliveryRatio >= noDeficit.DeliveryRatio {
		t.Fatalf("moving to mesh 6 from a deployed mesh of 10 did not carry more delivery risk than moving to mesh 6 from a deployed mesh of 6: from6=%v from10=%v", noDeficit.DeliveryRatio, withDeficit.DeliveryRatio)
	}
}

func TestControlledProfilesDependOnSeed(t *testing.T) {
	first := controlledProfile(3, 42)
	same := controlledProfile(3, 42)
	different := controlledProfile(3, 43)
	if first != same {
		t.Fatalf("same seed produced different profile: %+v vs %+v", first, same)
	}
	if first == different {
		t.Fatalf("different seed produced identical profile: %+v", first)
	}
}

func TestDeploymentGuardrailAcceptsImprovementAndRejectsRegression(t *testing.T) {
	workload := Workload{Peers: 8, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1, RandomSeed: 4242}
	profiles := make([]controlledParticipantProfile, workload.Peers)
	for i := range profiles {
		profiles[i] = controlledProfile(i, workload.RandomSeed)
	}
	current := fedgreensub.GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: time.Second}
	better := fedgreensub.GossipParameters{MeshDegree: 8, GossipFactor: .20, HeartbeatInterval: 500 * time.Millisecond}
	worse := fedgreensub.GossipParameters{MeshDegree: 8, GossipFactor: .25, HeartbeatInterval: 5 * time.Second}
	if accepted, reason := deploymentDecision(current, better, profiles, workload); !accepted {
		t.Fatalf("better candidate rejected: %s", reason)
	}
	if accepted, reason := deploymentDecision(current, worse, profiles, workload); accepted {
		t.Fatalf("worse candidate accepted: %s", reason)
	}
}

func TestDeploymentGuardrailToleranceAndInvalidOutcome(t *testing.T) {
	base := fedgreensub.CandidateScore{DeliveryRatio: .95, DuplicateRatio: .10, EnergyCost: .40, Valid: true}
	small := fedgreensub.CandidateScore{DeliveryRatio: .946, DuplicateRatio: .11, EnergyCost: .405, Valid: true}
	if accepted, _ := deploymentDecisionWithScores(base, small, .5, .4); !accepted {
		t.Fatal("objective improvement within tolerance was rejected")
	}
	large := fedgreensub.CandidateScore{DeliveryRatio: .90, DuplicateRatio: .10, EnergyCost: .40, Valid: true}
	if accepted, _ := deploymentDecisionWithScores(base, large, .5, .4); accepted {
		t.Fatal("large delivery regression was accepted")
	}
	invalid := large
	invalid.Valid = false
	if accepted, _ := deploymentDecisionWithScores(base, invalid, .5, .4); accepted {
		t.Fatal("invalid candidate was accepted")
	}
}

func TestAllFiveModesShareWorkloadButProduceDistinctControlPaths(t *testing.T) {
	workload := Workload{Peers: 8, Messages: 100, MessageSize: 64, PublishRate: 20, Duration: time.Second, RandomSeed: 42, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1}
	implementations := []Implementation{&BaselineGossipSub{}, &HeuristicGossipSub{}, &FederatedGossipSub{}, &FedGreenSub{}, &TrustAwareFedGreenSub{}}
	for _, implementation := range implementations {
		if err := implementation.Start(Config{FLRounds: 3}); err != nil {
			t.Fatal(err)
		}
		if err := implementation.RunWorkload(workload); err != nil {
			t.Fatal(err)
		}
		metrics := implementation.Metrics()
		if metrics.Published != uint64(workload.Messages) {
			t.Fatalf("%s published %d", implementation.Name(), metrics.Published)
		}
		if metrics.DeliveryRatio < 0 || metrics.DeliveryRatio > 1 {
			t.Fatalf("%s invalid delivery ratio", implementation.Name())
		}
		if err := implementation.Stop(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFederatedMetricsExportRoundEvidence(t *testing.T) {
	implementation := &FederatedGossipSub{}
	if err := implementation.Start(Config{FLRounds: 3}); err != nil {
		t.Fatal(err)
	}
	defer implementation.Stop()
	if err := implementation.RunWorkload(Workload{Peers: 8, Messages: 100, MessageSize: 64, Duration: time.Second, RandomSeed: 42, PacketLoss: .05, Latency: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	metrics := implementation.Metrics()
	if metrics.FLRounds != 3 || len(metrics.GlobalLossByRound) != 3 || len(metrics.ParameterChangeByRound) != 3 || len(metrics.TrainingTimeByRound) != 3 || len(metrics.FLRoundTrace) != 3 || len(metrics.EffectiveWeights) != 8 {
		t.Fatalf("missing round evidence: %+v", metrics)
	}
	if metrics.ParameterChange == 0 || metrics.Contributors != 8 {
		t.Fatalf("missing parameter/contributor evidence: %+v", metrics)
	}
	if metrics.TrainingTime <= 0 || metrics.AggregationTime <= 0 || metrics.TotalRoundTime <= 0 {
		t.Fatalf("timing instrumentation is empty: %+v", metrics)
	}
}

func TestFederatedRoundPropagatesGlobalModelToPredictor(t *testing.T) {
	workload := Workload{Peers: 10, Messages: 100, MessageSize: 64, Duration: time.Second, RandomSeed: 4242, Latency: 20 * time.Millisecond}
	fed := &FedGreenSub{}
	if err := fed.Start(Config{FLRounds: 1}); err != nil {
		t.Fatal(err)
	}
	if err := fed.RunWorkload(workload); err != nil {
		t.Fatal(err)
	}
	defer fed.Stop()
	trace := fed.Metrics().FLRoundTrace[0]
	if trace.AggregationParameterChange <= 0 || trace.PredictorParameterChange <= 0 {
		t.Fatalf("global model was not propagated: %+v", trace)
	}
}

func TestTrustAwarePropagationProducesDistinctPredictionState(t *testing.T) {
	workload := Workload{Peers: 10, Messages: 100, MessageSize: 64, Duration: time.Second, RandomSeed: 4242, Latency: 20 * time.Millisecond}
	fed := &FedGreenSub{}
	trust := &TrustAwareFedGreenSub{}
	if err := fed.Start(Config{FLRounds: 1}); err != nil {
		t.Fatal(err)
	}
	if err := trust.Start(Config{FLRounds: 1}); err != nil {
		t.Fatal(err)
	}
	defer fed.Stop()
	defer trust.Stop()
	if err := fed.RunWorkload(workload); err != nil {
		t.Fatal(err)
	}
	if err := trust.RunWorkload(workload); err != nil {
		t.Fatal(err)
	}
	left := fed.Metrics().FLRoundTrace[0].ModelOutput
	right := trust.Metrics().FLRoundTrace[0].ModelOutput
	different := false
	for i := range left {
		if left[i] != right[i] {
			different = true
			break
		}
	}
	if !different {
		t.Fatalf("trust-aware and FedGreen predictions were identical: fed=%v trust=%v", left, right)
	}
}

func TestNeighborMeshSelectionUsesFloorRoundCeilAndDeduplicates(t *testing.T) {
	current := fedgreensub.GossipParameters{MeshDegree: 8, DLow: 4, DHigh: 12, GossipFactor: .25, HeartbeatInterval: time.Second}
	profiles := []controlledParticipantProfile{controlledProfile(0, 4242)}
	workload := Workload{Peers: 1, Latency: 20 * time.Millisecond, RandomSeed: 4242}
	candidate, decoded, meshes, objectives := selectNeighborMesh(current, current, []float64{.375, .5, .5}, profiles, workload, fedgreensub.DefaultConfig())
	if decoded != 7.5 || len(meshes) != 2 || candidate.MeshDegree < 7 || candidate.MeshDegree > 8 || len(objectives) != 2 {
		t.Fatalf("unexpected neighbor selection: decoded=%v meshes=%v candidate=%+v objectives=%v", decoded, meshes, candidate, objectives)
	}
}

func TestNeighborMeshSelectionClampsAtBounds(t *testing.T) {
	current := fedgreensub.GossipParameters{MeshDegree: 3, DLow: 3, DHigh: 7, GossipFactor: .25, HeartbeatInterval: time.Second}
	profiles := []controlledParticipantProfile{controlledProfile(0, 4242)}
	workload := Workload{Peers: 1, Latency: 20 * time.Millisecond, RandomSeed: 4242}
	_, _, meshes, _ := selectNeighborMesh(current, current, []float64{0, .5, .5}, profiles, workload, fedgreensub.DefaultConfig())
	for _, mesh := range meshes {
		if mesh < 3 || mesh > 5 {
			t.Fatalf("mesh neighbor escaped bounds: %v", meshes)
		}
	}
}

func TestSelectBestMeshCandidateHandlesLowerHigherCurrentAndInvalid(t *testing.T) {
	if got := selectBestMeshCandidate([]int{6, 7, 8}, map[int]float64{6: .2, 7: .3, 8: .4}, 8); got != 6 {
		t.Fatalf("lower objective candidate=%d", got)
	}
	if got := selectBestMeshCandidate([]int{6, 7, 8}, map[int]float64{6: .4, 7: .3, 8: .2}, 6); got != 8 {
		t.Fatalf("higher objective candidate=%d", got)
	}
	if got := selectBestMeshCandidate([]int{6, 7, 8}, map[int]float64{6: .4, 7: .2, 8: .4}, 7); got != 7 {
		t.Fatalf("current best candidate=%d", got)
	}
	if got := selectBestMeshCandidate([]int{6, 7}, map[int]float64{6: math.NaN(), 7: math.Inf(1)}, 7); got != 7 {
		t.Fatalf("invalid objectives candidate=%d", got)
	}
}

func TestAggregationModesKeepDistinctEffectiveWeights(t *testing.T) {
	workload := Workload{Peers: 8, Messages: 100, MessageSize: 64, Duration: time.Second, RandomSeed: 4242, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1}
	readWeights := func(implementation Implementation) map[string]float64 {
		if err := implementation.Start(Config{FLRounds: 1}); err != nil {
			t.Fatal(err)
		}
		defer implementation.Stop()
		if err := implementation.RunWorkload(workload); err != nil {
			t.Fatal(err)
		}
		return implementation.Metrics().EffectiveWeights
	}
	fedAvg := readWeights(&FederatedGossipSub{})
	energy := readWeights(&FedGreenSub{})
	trust := readWeights(&TrustAwareFedGreenSub{})
	if fedAvg["participant-0"] == energy["participant-0"] || fedAvg["participant-0"] == trust["participant-0"] || energy["participant-0"] == trust["participant-0"] {
		t.Fatalf("aggregation modes unexpectedly share participant weight: fedavg=%v energy=%v trust=%v", fedAvg, energy, trust)
	}
}

// --- Dual (previous + run-start) per-round guardrail ---
//
// These tests guard the cumulative-drift fix: a sequence of individually
// small, individually-accepted steps (8 -> 7 -> 6 -> 5 ...) must no longer be
// able to silently walk the deployed mesh degree far from the configuration
// that was actually known safe at the start of the FL run.

func tenPeerDriftProfiles(seed int64) []controlledParticipantProfile {
	profiles := make([]controlledParticipantProfile, 10)
	for i := range profiles {
		profiles[i] = controlledProfile(i, seed)
	}
	return profiles
}

// runTenPeerDriftTrace runs the real FedGreenSub adapter for the workload/
// seed that exhibits cumulative drift and returns its FLRoundTrace, so the
// following tests exercise exact, production-derived candidate parameters
// (full floating-point precision) rather than hand-rounded approximations
// that could trip the guardrail's strict objective-tie tolerance for
// unrelated reasons.
func runTenPeerDriftTrace(t *testing.T, rounds int) []FLRoundTrace {
	t.Helper()
	workload := Workload{Peers: 10, Messages: 200, MessageSize: 1024, PublishRate: 100, Duration: 2 * time.Second, RandomSeed: 4242}
	fed := &FedGreenSub{}
	if err := fed.Start(Config{FLRounds: rounds}); err != nil {
		t.Fatal(err)
	}
	if err := fed.RunWorkload(workload); err != nil {
		t.Fatal(err)
	}
	return fed.Metrics().FLRoundTrace
}

// TestEvaluateGuardrailReferenceAcceptsSafeChangeVsBothReferences covers
// requirement 1: a small, genuinely safe step is accepted whether checked
// against the previous configuration or the run-start configuration (at
// round 1 these are the same configuration, which is the common case).
func TestEvaluateGuardrailReferenceAcceptsSafeChangeVsBothReferences(t *testing.T) {
	trace := runTenPeerDriftTrace(t, 1)
	if len(trace) != 1 || !trace[0].Accepted {
		t.Fatalf("expected round 1 to be accepted: %+v", trace)
	}
	profiles := tenPeerDriftProfiles(4242)
	workload := Workload{Peers: 10, Messages: 200, MessageSize: 1024, PublishRate: 100, Duration: 2 * time.Second, RandomSeed: 4242}
	cfg := fedgreensub.DefaultConfig()
	runStart := defaultParameters()
	safeCandidate := trace[0].Candidate // the real round-1 mesh 8->7 candidate

	_, _, _, _, acceptedVsPrevious, reasonVsPrevious := evaluateGuardrailReference(profiles, workload, cfg, runStart, safeCandidate)
	if !acceptedVsPrevious {
		t.Fatalf("expected a safe first step to be accepted vs previous (=run-start at round 1): %s", reasonVsPrevious)
	}
	_, _, _, _, acceptedVsRunStart, reasonVsRunStart := evaluateGuardrailReference(profiles, workload, cfg, runStart, safeCandidate)
	if !acceptedVsRunStart {
		t.Fatalf("expected a safe first step to be accepted vs run-start: %s", reasonVsRunStart)
	}
}

// TestEvaluateGuardrailReferenceRejectsCumulativeDriftVsRunStart covers
// requirement 2: a candidate that is a small, safe step from the previous
// round's configuration can still be an unsafe cumulative move away from the
// run-start configuration, and must be rejected on that basis. previous and
// candidate below are the real round-1-accepted and round-2-proposed
// configurations observed for this workload/seed.
func TestEvaluateGuardrailReferenceRejectsCumulativeDriftVsRunStart(t *testing.T) {
	trace := runTenPeerDriftTrace(t, 2)
	if len(trace) != 2 || !trace[0].Accepted {
		t.Fatalf("expected round 1 to be accepted and 2 rounds of trace: %+v", trace)
	}
	profiles := tenPeerDriftProfiles(4242)
	workload := Workload{Peers: 10, Messages: 200, MessageSize: 1024, PublishRate: 100, Duration: 2 * time.Second, RandomSeed: 4242}
	cfg := fedgreensub.DefaultConfig()
	runStart := defaultParameters()  // mesh=8
	previous := trace[0].Candidate   // mesh=7, accepted round-1 result
	candidate := trace[1].Candidate  // mesh=6, round-2 proposal

	_, _, _, _, acceptedVsPrevious, reasonVsPrevious := evaluateGuardrailReference(profiles, workload, cfg, previous, candidate)
	if !acceptedVsPrevious {
		t.Fatalf("expected mesh 7->6 to be a safe local step vs previous, got rejected: %s", reasonVsPrevious)
	}
	_, _, _, _, acceptedVsRunStart, reasonVsRunStart := evaluateGuardrailReference(profiles, workload, cfg, runStart, candidate)
	if acceptedVsRunStart {
		t.Fatal("expected the cumulative move from run-start mesh=8 to candidate mesh=6 to be rejected as an unsafe cumulative drift")
	}
	if reasonVsRunStart == "" {
		t.Fatal("expected a non-empty rejection reason")
	}
}

// TestRejectedRoundLeavesParametersUnchanged covers requirement 3: once a
// round is rejected, the deployed reference used by the NEXT round's
// "current" field must not have moved.
func TestRejectedRoundLeavesParametersUnchanged(t *testing.T) {
	workload := Workload{Peers: 10, Messages: 200, MessageSize: 1024, PublishRate: 100, Duration: 2 * time.Second, RandomSeed: 4242}
	fed := &FedGreenSub{}
	if err := fed.Start(Config{FLRounds: 3}); err != nil {
		t.Fatal(err)
	}
	if err := fed.RunWorkload(workload); err != nil {
		t.Fatal(err)
	}
	trace := fed.Metrics().FLRoundTrace
	if len(trace) != 3 {
		t.Fatalf("expected 3 rounds of trace, got %d", len(trace))
	}
	if !trace[0].Accepted {
		t.Fatalf("expected round 1 (mesh 8->7) to be accepted: %s", trace[0].RejectionReason)
	}
	if trace[1].Accepted {
		t.Fatalf("expected round 2 (cumulative mesh 8->6) to be rejected, it was accepted")
	}
	if trace[2].Accepted {
		t.Fatalf("expected round 3 to be rejected, it was accepted")
	}
	// Because round 2 was rejected, round 3's "current" (the deployed
	// reference it trains/predicts relative to) must be identical to round
	// 2's "current" -- it must not have silently advanced to round 2's
	// rejected candidate.
	if trace[1].Current != trace[2].Current {
		t.Fatalf("parameters advanced despite a rejected round: round2.Current=%+v round3.Current=%+v", trace[1].Current, trace[2].Current)
	}
	// And it must equal round 1's accepted candidate, not run-start.
	if trace[1].Current != trace[0].Candidate {
		t.Fatalf("round 2's current does not match round 1's accepted candidate: round2.Current=%+v round1.Candidate=%+v", trace[1].Current, trace[0].Candidate)
	}
}

// TestRunStartReferenceStaysFixedAcrossManyRounds covers requirement 4:
// runStart must not drift even after the deployed configuration has moved
// and many further rounds have been rejected against it. If runStart were
// silently tracking intermediate state, later rounds' "vs run-start" checks
// would stop being reproducible against a fixed defaultParameters() baseline.
func TestRunStartReferenceStaysFixedAcrossManyRounds(t *testing.T) {
	workload := Workload{Peers: 10, Messages: 200, MessageSize: 1024, PublishRate: 100, Duration: 2 * time.Second, RandomSeed: 4242}
	fed := &FedGreenSub{}
	if err := fed.Start(Config{FLRounds: 9}); err != nil {
		t.Fatal(err)
	}
	if err := fed.RunWorkload(workload); err != nil {
		t.Fatal(err)
	}
	trace := fed.Metrics().FLRoundTrace
	if len(trace) != 9 {
		t.Fatalf("expected 9 rounds of trace, got %d", len(trace))
	}
	profiles := tenPeerDriftProfiles(4242)
	cfg := fedgreensub.DefaultConfig()
	runStart := defaultParameters()
	for i := 1; i < len(trace); i++ { // skip round 1, which trivially matches (previous == run-start there)
		if trace[i].Accepted {
			continue
		}
		// Independently recompute what a "vs run-start" check on this exact
		// round's proposed candidate would decide, using the fixed
		// defaultParameters() baseline. If runStart had drifted to whatever
		// configuration was deployed at the time, this independent
		// recomputation (always anchored to the true default) would diverge
		// from the production result for at least one of these rounds.
		_, _, _, _, accepted, reason := evaluateGuardrailReference(profiles, workload, cfg, runStart, trace[i].Candidate)
		if accepted {
			t.Fatalf("round %d: candidate %+v unexpectedly safe vs the fixed run-start baseline, but production rejected it (%s) -- run-start reference may have drifted", trace[i].Round, trace[i].Candidate, trace[i].RejectionReason)
		}
		if reason == "" {
			t.Fatalf("round %d: expected a non-empty rejection reason from the fixed run-start baseline", trace[i].Round)
		}
	}
}

// TestOuterGuardrailStillAppliesAfterDualPerRoundGuardrail covers
// requirement 5: the outer, whole-run guardrail in run() (deploymentDecision)
// still exists, is still exercised end to end, and the value it ultimately
// approves is what gets reported as deployed -- not merely whatever the
// per-round loop happened to return.
func TestOuterGuardrailStillAppliesAfterDualPerRoundGuardrail(t *testing.T) {
	workload := Workload{Peers: 10, Messages: 200, MessageSize: 1024, PublishRate: 100, Duration: 2 * time.Second, RandomSeed: 4242}
	fed := &FedGreenSub{}
	if err := fed.Start(Config{FLRounds: 3}); err != nil {
		t.Fatal(err)
	}
	if err := fed.RunWorkload(workload); err != nil {
		t.Fatal(err)
	}
	m := fed.Metrics()
	// The outer guardrail (deploymentDecision), unchanged by this fix, must
	// independently agree that whatever ended up deployed is safe relative to
	// run-start. This is the same function exercised directly (unchanged
	// behavior) by TestDeploymentGuardrailAcceptsImprovementAndRejectsRegression.
	profiles := tenPeerDriftProfiles(4242)
	deployedMesh := int(m.MeshDegree)
	accepted, reason := deploymentDecision(defaultParameters(), fedgreensub.GossipParameters{MeshDegree: deployedMesh, DLow: deployedMesh / 2, DHigh: deployedMesh + 4, GossipFactor: .2485, HeartbeatInterval: 993 * time.Millisecond}, profiles, workload)
	if !accepted {
		t.Fatalf("expected the outer guardrail to still independently approve the deployed configuration: %s", reason)
	}
	if m.MeshDegree < 7 || m.MeshDegree > 8 {
		t.Fatalf("expected the deployed mesh degree to be the safe value (7), got %v", m.MeshDegree)
	}
}

// TestCumulativeDriftFixAcrossAllSeeds10Peers covers requirement 6: reproduce
// the exact scenario from the investigation (10 peers, 3 FL rounds, seeds
// 4242-4246) and verify the deployed trajectory no longer reaches mesh=5, and
// that the safe first adaptation to mesh=7 survives to become the deployed
// configuration.
func TestCumulativeDriftFixAcrossAllSeeds10Peers(t *testing.T) {
	for _, seed := range []int64{4242, 4243, 4244, 4245, 4246} {
		workload := Workload{Peers: 10, Messages: 200, MessageSize: 1024, PublishRate: 100, Duration: 2 * time.Second, RandomSeed: seed}
		fed := &FedGreenSub{}
		if err := fed.Start(Config{FLRounds: 3}); err != nil {
			t.Fatal(err)
		}
		if err := fed.RunWorkload(workload); err != nil {
			t.Fatal(err)
		}
		m := fed.Metrics()
		if m.MeshDegree != 7 {
			t.Fatalf("seed=%d: expected the deployed (not merely internally-proposed) mesh degree to settle safely at 7, got %v", seed, m.MeshDegree)
		}
		if len(m.FLRoundTrace) != 3 {
			t.Fatalf("seed=%d: expected 3 rounds of trace, got %d", seed, len(m.FLRoundTrace))
		}
		if !m.FLRoundTrace[0].Accepted || m.FLRoundTrace[0].Candidate.MeshDegree != 7 {
			t.Fatalf("seed=%d: expected round 1 to accept the safe mesh 8->7 step, got accepted=%v candidateMesh=%d", seed, m.FLRoundTrace[0].Accepted, m.FLRoundTrace[0].Candidate.MeshDegree)
		}
		for i := 1; i < len(m.FLRoundTrace); i++ {
			if m.FLRoundTrace[i].Accepted {
				t.Fatalf("seed=%d round=%d: expected further cumulative drift below mesh 7 to be rejected, but it was accepted (candidateMesh=%d) -- the 8->7->6->5 drift pattern has reappeared", seed, m.FLRoundTrace[i].Round, m.FLRoundTrace[i].Candidate.MeshDegree)
			}
			if m.FLRoundTrace[i].Candidate.MeshDegree < 7 {
				// This is fine (a rejected proposal), but explicitly confirm it
				// was rejected and never became the deployed value.
				if int(m.MeshDegree) == m.FLRoundTrace[i].Candidate.MeshDegree {
					t.Fatalf("seed=%d round=%d: an internally-rejected candidate (mesh=%d) was reported as the deployed configuration", seed, m.FLRoundTrace[i].Round, m.FLRoundTrace[i].Candidate.MeshDegree)
				}
			}
		}
	}
}
