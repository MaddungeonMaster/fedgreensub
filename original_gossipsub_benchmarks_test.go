package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

func setupOriginalGossipBench(b *testing.B, peerCount int) ([]host.Host, []*pubsub.PubSub, []*pubsub.Topic, []*pubsub.Subscription, context.Context, context.CancelFunc) {
	b.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	hosts := make([]host.Host, 0, peerCount)

	for i := 0; i < peerCount; i++ {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		if err != nil {
			cancel()
			b.Fatalf("create host %d: %v", i, err)
		}
		hosts = append(hosts, h)
	}

	for i := 1; i < len(hosts); i++ {
		target := peer.AddrInfo{ID: hosts[0].ID(), Addrs: hosts[0].Addrs()}
		hosts[i].Peerstore().AddAddrs(target.ID, target.Addrs, peerstore.PermanentAddrTTL)
		if err := hosts[i].Connect(ctx, target); err != nil {
			cancel()
			b.Fatalf("connect host %d to peer 0: %v", i, err)
		}
	}

	pss := make([]*pubsub.PubSub, peerCount)
	topics := make([]*pubsub.Topic, peerCount)
	subs := make([]*pubsub.Subscription, peerCount)

	for i, h := range hosts {
		ps, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			cancel()
			b.Fatalf("new pubsub for host %d: %v", i, err)
		}
		pss[i] = ps

		topic, err := ps.Join("bench-original-gossipsub")
		if err != nil {
			cancel()
			b.Fatalf("join topic on host %d: %v", i, err)
		}
		sub, err := topic.Subscribe()
		if err != nil {
			cancel()
			b.Fatalf("subscribe to topic on host %d: %v", i, err)
		}

		topics[i] = topic
		subs[i] = sub
	}

	time.Sleep(500 * time.Millisecond)
	return hosts, pss, topics, subs, ctx, cancel
}

func closeOriginalGossipBench(hosts []host.Host, cancel context.CancelFunc) {
	cancel()
	for _, h := range hosts {
		_ = h.Close()
	}
}

func BenchmarkOriginalGossipSubPublish(b *testing.B) {
	hosts, _, topics, subs, ctx, cancel := setupOriginalGossipBench(b, 3)
	defer closeOriginalGossipBench(hosts, cancel)

	if len(subs) < 2 {
		b.Fatalf("expected at least 2 subscriptions, got %d", len(subs))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload := []byte(fmt.Sprintf("gossipbench-%d", i))
		if err := topics[0].Publish(ctx, payload); err != nil {
			b.Fatalf("publish message: %v", err)
		}
		if _, err := subs[1].Next(ctx); err != nil {
			b.Fatalf("receive published message: %v", err)
		}
	}
}

func BenchmarkOriginalGossipSubConcurrentPublish(b *testing.B) {
	hosts, _, topics, subs, ctx, cancel := setupOriginalGossipBench(b, 5)
	defer closeOriginalGossipBench(hosts, cancel)

	if len(subs) < 2 {
		b.Fatalf("expected at least 2 subscriptions, got %d", len(subs))
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			payload := []byte(fmt.Sprintf("concurrent-%d", i))
			if err := topics[0].Publish(ctx, payload); err != nil {
				b.Fatalf("publish message: %v", err)
			}
			if _, err := subs[1].Next(ctx); err != nil {
				b.Fatalf("receive published message: %v", err)
			}
			i++
		}
	})
}
