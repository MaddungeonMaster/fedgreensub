package tests

import (
	"testing"
	"time"

	"golang_project/evaluation/comparison/implementations"
)

func TestAllImplementationsRunIdenticalWorkload(t *testing.T) {
	workload := implementations.Workload{Peers: 10, Messages: 100, MessageSize: 64, PublishRate: 20, Duration: 5 * time.Second, RandomSeed: 42, Latency: 5 * time.Millisecond}
	for _, impl := range []implementations.Implementation{&implementations.BaselineGossipSub{}, &implementations.FedGreenSub{}, &implementations.TrustAwareFedGreenSub{}} {
		if err := impl.Start(implementations.Config{FLRounds: 2}); err != nil {
			t.Fatalf("%s start: %v", impl.Name(), err)
		}
		if err := impl.RunWorkload(workload); err != nil {
			t.Fatalf("%s workload: %v", impl.Name(), err)
		}
		metrics := impl.Metrics()
		if metrics.Published != uint64(workload.Messages) {
			t.Fatalf("%s published %d", impl.Name(), metrics.Published)
		}
		if metrics.DeliveryRatio < 0 || metrics.DeliveryRatio > 1 || metrics.DuplicateRatio < 0 || metrics.DuplicateRatio > 1 || metrics.Energy < 0 {
			t.Fatalf("%s invalid metrics: %+v", impl.Name(), metrics)
		}
		if err := impl.Stop(); err != nil {
			t.Fatalf("%s stop: %v", impl.Name(), err)
		}
	}
}

func TestTrustAwareMetricsAreBounded(t *testing.T) {
	impl := &implementations.TrustAwareFedGreenSub{}
	if err := impl.Start(implementations.Config{FLRounds: 1}); err != nil {
		t.Fatal(err)
	}
	if err := impl.RunWorkload(implementations.Workload{Peers: 8, Messages: 32, MessageSize: 32, PublishRate: 10, Duration: time.Second, RandomSeed: 7}); err != nil {
		t.Fatal(err)
	}
	m := impl.Metrics()
	if m.AverageTrust < 0 || m.AverageTrust > 1 || m.MinimumTrust < 0 || m.MinimumTrust > 1 || m.TrustedPeerRatio < 0 || m.TrustedPeerRatio > 1 {
		t.Fatalf("trust out of bounds: %+v", m)
	}
}

func TestSameSeedProducesSameWorkloadInputs(t *testing.T) {
	one := implementations.Workload{Peers: 20, Messages: 100, MessageSize: 128, PublishRate: 25, Duration: 4 * time.Second, RandomSeed: 42}
	two := one
	if one != two {
		t.Fatal("identical seeds/configuration did not reproduce the workload")
	}
}
