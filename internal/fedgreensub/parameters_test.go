package fedgreensub

import (
	"testing"
	"time"
)

func TestValidateValidParameters(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	params := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}

	report := manager.Validate(params)

	if !report.Accepted {
		t.Fatalf(
			"expected valid parameters to be accepted, got: %s",
			report.Reason,
		)
	}
}

func TestRejectMeshDegreeBelowMinimum(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	params := GossipParameters{
		MeshDegree:        2,
		DLow:              1,
		DHigh:             4,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}

	report := manager.Validate(params)

	if report.Accepted {
		t.Fatal(
			"expected parameters with a low mesh degree to be rejected",
		)
	}
}

func TestRejectMeshDegreeAboveMaximum(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	params := GossipParameters{
		MeshDegree:        20,
		DLow:              4,
		DHigh:             24,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}

	report := manager.Validate(params)

	if report.Accepted {
		t.Fatal(
			"expected parameters with a high mesh degree to be rejected",
		)
	}
}

func TestRejectInvalidDegreeRelationship(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	params := GossipParameters{
		MeshDegree:        8,
		DLow:              10,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}

	report := manager.Validate(params)

	if report.Accepted {
		t.Fatal(
			"expected DLow greater than MeshDegree to be rejected",
		)
	}
}

func TestRejectInvalidGossipFactor(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	params := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.8,
		HeartbeatInterval: time.Second,
	}

	report := manager.Validate(params)

	if report.Accepted {
		t.Fatal(
			"expected an out-of-range gossip factor to be rejected",
		)
	}
}

func TestRejectInvalidHeartbeatInterval(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	params := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: 100 * time.Millisecond,
	}

	report := manager.Validate(params)

	if report.Accepted {
		t.Fatal(
			"expected a heartbeat interval below the minimum to be rejected",
		)
	}
}

func TestApplyParameters(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewParameterManager(cfg)

	params := GossipParameters{
		MeshDegree:        8,
		DLow:              4,
		DHigh:             12,
		GossipFactor:      0.25,
		HeartbeatInterval: time.Second,
	}

	report := manager.ApplyParameters(params)

	if !report.Accepted {
		t.Fatalf(
			"expected parameters to be applied, got: %s",
			report.Reason,
		)
	}

	current := manager.CurrentParameters()

	if current.MeshDegree != params.MeshDegree {
		t.Errorf(
			"expected MeshDegree %d, got %d",
			params.MeshDegree,
			current.MeshDegree,
		)
	}

	if current.DLow != params.DLow {
		t.Errorf(
			"expected DLow %d, got %d",
			params.DLow,
			current.DLow,
		)
	}

	if current.DHigh != params.DHigh {
		t.Errorf(
			"expected DHigh %d, got %d",
			params.DHigh,
			current.DHigh,
		)
	}

	if current.GossipFactor != params.GossipFactor {
		t.Errorf(
			"expected GossipFactor %.2f, got %.2f",
			params.GossipFactor,
			current.GossipFactor,
		)
	}

	if current.HeartbeatInterval != params.HeartbeatInterval {
		t.Errorf(
			"expected HeartbeatInterval %v, got %v",
			params.HeartbeatInterval,
			current.HeartbeatInterval,
		)
	}
}
