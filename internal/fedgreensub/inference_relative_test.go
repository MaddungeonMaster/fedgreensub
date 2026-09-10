package fedgreensub

import (
	"math"
	"testing"
	"time"
)

func TestRelativeParameterTargetRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	current := GossipParameters{MeshDegree: 8, DLow: 4, DHigh: 12, GossipFactor: .25, HeartbeatInterval: time.Second}
	cases := []GossipParameters{
		{MeshDegree: 6, DLow: 3, DHigh: 10, GossipFactor: .20, HeartbeatInterval: 750 * time.Millisecond},
		current,
		{MeshDegree: 10, DLow: 5, DHigh: 14, GossipFactor: .30, HeartbeatInterval: 1250 * time.Millisecond},
	}
	for _, want := range cases {
		target := ParametersTarget(want, cfg, current)
		got, err := ParametersFromModelOutput(target, cfg, current)
		if err != nil {
			t.Fatal(err)
		}
		if got.MeshDegree != want.MeshDegree || got.GossipFactor != want.GossipFactor || got.HeartbeatInterval != want.HeartbeatInterval {
			t.Fatalf("round trip target=%v got=%+v want=%+v", target, got, want)
		}
	}
}

func TestRelativeParameterMidpointIsCurrent(t *testing.T) {
	cfg := DefaultConfig()
	current := GossipParameters{MeshDegree: 8, DLow: 4, DHigh: 12, GossipFactor: .25, HeartbeatInterval: time.Second}
	got, err := ParametersFromModelOutput([]float64{.5, .5, .5}, cfg, current)
	if err != nil {
		t.Fatal(err)
	}
	if got.MeshDegree != current.MeshDegree || got.GossipFactor != current.GossipFactor || got.HeartbeatInterval != current.HeartbeatInterval {
		t.Fatalf("midpoint=%+v current=%+v", got, current)
	}
}

func TestRelativeParameterTargetsAreBounded(t *testing.T) {
	cfg := DefaultConfig()
	current := GossipParameters{MeshDegree: 8, DLow: 4, DHigh: 12, GossipFactor: .25, HeartbeatInterval: time.Second}
	for _, output := range [][]float64{{0, 0, 0}, {1, 1, 1}} {
		got, err := ParametersFromModelOutput(output, cfg, current)
		if err != nil {
			t.Fatal(err)
		}
		if got.MeshDegree < cfg.MinMeshDegree || got.MeshDegree > cfg.MaxMeshDegree || got.HeartbeatInterval < cfg.MinHeartbeatInterval || got.HeartbeatInterval > cfg.MaxHeartbeatInterval || math.IsNaN(got.GossipFactor) {
			t.Fatalf("decoded out of bounds: %+v", got)
		}
	}
}

func TestCompareCandidateDeploymentGuardrail(t *testing.T) {
	cfg := DefaultConfig()
	current := CandidateScore{DeliveryRatio: .95, DuplicateRatio: .10, EnergyCost: .40, Valid: true}
	better := CandidateScore{DeliveryRatio: .95, DuplicateRatio: .09, EnergyCost: .39, Valid: true}
	if report := compareCandidateDeployment(current, better, cfg); !report.Accepted {
		t.Fatalf("better candidate rejected: %+v", report)
	}
	worse := CandidateScore{DeliveryRatio: .90, DuplicateRatio: .10, EnergyCost: .40, Valid: true}
	if report := compareCandidateDeployment(current, worse, cfg); report.Accepted {
		t.Fatalf("worse candidate accepted: %+v", report)
	}
}
