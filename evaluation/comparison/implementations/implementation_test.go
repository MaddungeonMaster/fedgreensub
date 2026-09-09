package implementations

import (
	"testing"
	"time"

	fedgreensub "golang_project/internal/fedgreensub"
)

func TestControlledCandidateChangesNetworkOutcome(t *testing.T) {
	profile := controlledProfile(2, 42)
	workload := Workload{Peers: 8, Messages: 200, MessageSize: 128, PacketLoss: .05, Latency: 20 * time.Millisecond, Duplicates: .1}
	low := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 5, DLow: 3, DHigh: 9, GossipFactor: .1, HeartbeatInterval: 2 * time.Second})
	high := controlledOutcome(profile, workload, fedgreensub.GossipParameters{MeshDegree: 12, DLow: 6, DHigh: 16, GossipFactor: .4, HeartbeatInterval: 500 * time.Millisecond})
	if low.DeliveryRatio == high.DeliveryRatio && low.DuplicateRatio == high.DuplicateRatio && low.LatencyCost == high.LatencyCost && low.EnergyCost == high.EnergyCost {
		t.Fatal("candidate parameters did not change controlled network outcome")
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
	if metrics.FLRounds != 3 || len(metrics.GlobalLossByRound) != 3 || len(metrics.ParameterChangeByRound) != 3 || len(metrics.TrainingTimeByRound) != 3 || len(metrics.EffectiveWeights) != 8 {
		t.Fatalf("missing round evidence: %+v", metrics)
	}
	if metrics.ParameterChange == 0 || metrics.Contributors != 8 {
		t.Fatalf("missing parameter/contributor evidence: %+v", metrics)
	}
}
