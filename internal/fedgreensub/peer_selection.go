package fedgreensub

import (
	"sort"

	"github.com/libp2p/go-libp2p/core/peer"
)

// RankPeers ranks only caller-controlled choices. It does not alter
// GossipSub's peer score, mesh protocol, or eligibility rules.
func RankPeers(peers []peer.ID, trustScores map[peer.ID]TrustScore, gossipScores map[peer.ID]float64, cfg Config) []peer.ID {
	if len(peers) < 2 {
		return append([]peer.ID(nil), peers...)
	}
	cfg.normalize()
	ranked := append([]peer.ID(nil), peers...)
	sort.SliceStable(ranked, func(i, j int) bool {
		left, right := .5, .5
		if score, ok := trustScores[ranked[i]]; ok {
			left = score.Score
		}
		if score, ok := trustScores[ranked[j]]; ok {
			right = score.Score
		}
		left = CombinedPeerQuality(gossipScores[ranked[i]], left, cfg.PeerScoreWeight, cfg.TrustScoreWeight)
		right = CombinedPeerQuality(gossipScores[ranked[j]], right, cfg.PeerScoreWeight, cfg.TrustScoreWeight)
		return left > right
	})
	return ranked
}
