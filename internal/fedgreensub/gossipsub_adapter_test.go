package fedgreensub

import (
	"context"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

func TestGossipSubParameterApplierUpdatesRunningRouter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	router := pubsub.DefaultGossipSubRouter(host)
	ps, err := pubsub.NewGossipSubWithRouter(ctx, host, router)
	if err != nil {
		t.Fatal(err)
	}
	_ = ps

	cfg := NewConfig()
	manager := NewParameterManager(cfg)
	manager.SetApplier(NewGossipSubParameterApplier(router))
	parameters := GossipParameters{MeshDegree: 6, DLow: 3, DHigh: 10, GossipFactor: .2, HeartbeatInterval: 750 * time.Millisecond}
	if report := manager.ApplyParameters(parameters); !report.Accepted {
		t.Fatalf("parameter application rejected: %+v", report)
	}
	active, err := router.GossipSubParams(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.D != parameters.MeshDegree || active.Dlo != parameters.DLow || active.Dhi != parameters.DHigh || active.GossipFactor != parameters.GossipFactor || active.HeartbeatInterval != parameters.HeartbeatInterval {
		t.Fatalf("active router parameters = %+v, want adaptation", active)
	}
}
