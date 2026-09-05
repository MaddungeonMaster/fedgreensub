package fedgreensub

import (
	"math"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TrustScore is an observed behavioral signal. It is separate from
// GossipSub's internal peer score and is never sent on the wire.
type TrustScore struct {
	PeerID                                                                  peer.ID
	Reliability, Forwarding, Stability, PeerScore, LinkQuality, Misbehavior float64
	Score                                                                   float64
	Observations                                                            uint64
	LastUpdated                                                             time.Time
}

// PeerObservation contains only behavior visible to the local node.
type PeerObservation struct {
	PeerID                                             peer.ID
	MessageReceived, MessageForwarded, DeliverySuccess bool
	Duplicate, Invalid                                 bool
	Latency                                            time.Duration
	PacketLoss                                         float64
	Connected                                          bool
	Timestamp                                          time.Time
	PeerScore                                          float64
}

type TrustWeights struct {
	Reliability, Forwarding, Stability, PeerScore, LinkQuality, Misbehavior float64
}

func DefaultTrustWeights() TrustWeights {
	return TrustWeights{Reliability: .25, Forwarding: .15, Stability: .2, PeerScore: .15, LinkQuality: .15, Misbehavior: .1}
}

func (w TrustWeights) normalize() TrustWeights {
	if w.Reliability < 0 || w.Forwarding < 0 || w.Stability < 0 || w.PeerScore < 0 || w.LinkQuality < 0 || w.Misbehavior < 0 {
		return DefaultTrustWeights()
	}
	total := w.Reliability + w.Forwarding + w.Stability + w.PeerScore + w.LinkQuality + w.Misbehavior
	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return DefaultTrustWeights()
	}
	return TrustWeights{w.Reliability / total, w.Forwarding / total, w.Stability / total, w.PeerScore / total, w.LinkQuality / total, w.Misbehavior / total}
}

type TrustManager interface {
	Observe(peer.ID, PeerObservation)
	Score(peer.ID) float64
	Get(peer.ID) (TrustScore, bool)
	AllScores() map[peer.ID]TrustScore
	Remove(peer.ID)
}

type trustEntry struct {
	score                                                                   TrustScore
	reliability, forwarding, stability, peerScore, linkQuality, misbehavior float64
}

// PeerTrustManager maintains bounded, incrementally updated trust state.
type PeerTrustManager struct {
	mu             sync.RWMutex
	peers          map[peer.ID]*trustEntry
	weights        TrustWeights
	alpha, neutral float64
}

func NewTrustManager(weights TrustWeights, alpha float64) *PeerTrustManager {
	if alpha <= 0 || alpha > 1 || math.IsNaN(alpha) {
		alpha = .25
	}
	return &PeerTrustManager{peers: make(map[peer.ID]*trustEntry), weights: weights.normalize(), alpha: alpha, neutral: .5}
}

func (m *PeerTrustManager) Observe(id peer.ID, observation PeerObservation) {
	if m == nil || id == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.peers[id]
	if entry == nil {
		entry = &trustEntry{
			score:       TrustScore{PeerID: id, Reliability: m.neutral, Forwarding: m.neutral, Stability: m.neutral, PeerScore: m.neutral, LinkQuality: m.neutral, Misbehavior: m.neutral, Score: m.neutral},
			reliability: m.neutral,
			forwarding:  m.neutral,
			stability:   m.neutral,
			peerScore:   m.neutral,
			linkQuality: m.neutral,
			misbehavior: m.neutral,
		}
		m.peers[id] = entry
	}
	update := func(previous *float64, value float64) { *previous = m.alpha*clamp01(value) + (1-m.alpha)*(*previous) }
	reliability := 0.0
	if observation.DeliverySuccess {
		reliability = 1
	} else if observation.MessageReceived {
		reliability = .75
	}
	forwarding := 0.0
	if observation.MessageForwarded {
		forwarding = 1
	}
	stability := 0.0
	if observation.Connected {
		stability = 1
	}
	link := (1 - clamp01(observation.PacketLoss))
	if observation.Latency > 0 {
		link *= 1 / (1 + observation.Latency.Seconds())
	}
	misbehavior := 1.0
	if observation.Invalid {
		misbehavior -= .75
	}
	if observation.Duplicate {
		misbehavior -= .1
	}
	update(&entry.reliability, reliability)
	update(&entry.forwarding, forwarding)
	update(&entry.stability, stability)
	if observation.PeerScore != 0 {
		update(&entry.peerScore, normalizePeerScore(observation.PeerScore))
	}
	update(&entry.linkQuality, link)
	update(&entry.misbehavior, misbehavior)
	entry.score.Reliability, entry.score.Forwarding, entry.score.Stability = entry.reliability, entry.forwarding, entry.stability
	entry.score.PeerScore, entry.score.LinkQuality, entry.score.Misbehavior = entry.peerScore, entry.linkQuality, entry.misbehavior
	entry.score.Score = clamp01(m.weights.Reliability*entry.reliability + m.weights.Forwarding*entry.forwarding + m.weights.Stability*entry.stability + m.weights.PeerScore*entry.peerScore + m.weights.LinkQuality*entry.linkQuality + m.weights.Misbehavior*entry.misbehavior)
	entry.score.Observations++
	entry.score.LastUpdated = observation.Timestamp
	if entry.score.LastUpdated.IsZero() {
		entry.score.LastUpdated = time.Now()
	}
}

func (m *PeerTrustManager) Score(id peer.ID) float64 {
	if m == nil {
		return .5
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if e := m.peers[id]; e != nil {
		return e.score.Score
	}
	return m.neutral
}
func (m *PeerTrustManager) Get(id peer.ID) (TrustScore, bool) {
	if m == nil {
		return TrustScore{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.peers[id]
	if !ok {
		return TrustScore{}, false
	}
	return e.score, true
}
func (m *PeerTrustManager) AllScores() map[peer.ID]TrustScore {
	out := map[peer.ID]TrustScore{}
	if m == nil {
		return out
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, e := range m.peers {
		out[id] = e.score
	}
	return out
}
func (m *PeerTrustManager) Remove(id peer.ID) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.peers, id)
	m.mu.Unlock()
}

func clamp01(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return .5
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
func normalizePeerScore(value float64) float64 { return clamp01((value + 1) / 2) }

// CombinedPeerQuality is a ranking signal for FedGreenSub-controlled choices.
func CombinedPeerQuality(peerScore, trust, peerScoreWeight, trustWeight float64) float64 {
	total := peerScoreWeight + trustWeight
	if total <= 0 {
		return clamp01(trust)
	}
	return clamp01((peerScoreWeight*normalizePeerScore(peerScore) + trustWeight*trust) / total)
}
