package fedgreensub

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// GossipSubParameterApplier maps the extension's small adaptive surface onto
// the full router configuration while preserving construction-only fields.
type GossipSubParameterApplier struct {
	router *pubsub.GossipSubRouter
}

func NewGossipSubParameterApplier(router *pubsub.GossipSubRouter) *GossipSubParameterApplier {
	return &GossipSubParameterApplier{router: router}
}

func (a *GossipSubParameterApplier) ApplyGossipParameters(ctx context.Context, parameters GossipParameters) error {
	if a == nil || a.router == nil {
		return errNilGossipSubRouter
	}
	current, err := a.router.GossipSubParams(ctx)
	if err != nil {
		return err
	}
	current.D = parameters.MeshDegree
	current.Dlo = parameters.DLow
	current.Dhi = parameters.DHigh
	current.GossipFactor = parameters.GossipFactor
	current.HeartbeatInterval = parameters.HeartbeatInterval
	if parameters.FanoutTTL > 0 {
		current.FanoutTTL = parameters.FanoutTTL
	}
	if parameters.PruneBackoff > 0 {
		current.PruneBackoff = parameters.PruneBackoff
	}
	return a.router.UpdateGossipSubParams(ctx, current)
}

var errNilGossipSubRouter = errorString("fedgreensub: nil GossipSub router")

type errorString string

func (e errorString) Error() string { return string(e) }
