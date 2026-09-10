package fedgreensub

import (
	"context"
	"errors"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// LiveGossipProbe reports values exposed by the public PubSub API. Counters
// not exposed by v0.17.0 remain zero and are not presented as measurements.
type LiveGossipProbe struct {
	pubsub *pubsub.PubSub
	topic  string
	router *pubsub.GossipSubRouter
}

func NewLiveGossipProbe(ps *pubsub.PubSub, topic string, router *pubsub.GossipSubRouter) *LiveGossipProbe {
	return &LiveGossipProbe{pubsub: ps, topic: topic, router: router}
}

func (p *LiveGossipProbe) Snapshot(ctx context.Context) (GossipSample, error) {
	if p == nil || p.pubsub == nil {
		return GossipSample{}, errors.New("fedgreensub: nil live GossipSub probe")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return GossipSample{}, ctx.Err()
	default:
	}
	peers := p.pubsub.ListPeers(p.topic)
	sample := GossipSample{Timestamp: time.Now(), PeerCount: len(peers)}
	if p.router != nil {
		params, err := p.router.GossipSubParams(ctx)
		if err != nil {
			return GossipSample{}, err
		}
		// v0.17.0 does not expose actual per-topic mesh membership. This is
		// the configured target, not a measured mesh size.
		sample.MeshDegree = params.D
	}
	return sample, nil
}
