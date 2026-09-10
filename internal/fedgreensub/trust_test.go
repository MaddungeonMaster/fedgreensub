package fedgreensub

import (
	"math"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestTrustManagerEMAAndBounds(t *testing.T) {
	id := peer.ID("peer-a")
	m := NewTrustManager(DefaultTrustWeights(), .5)
	if got := m.Score(id); got != .5 {
		t.Fatalf("unknown peer trust = %v, want neutral", got)
	}
	m.Observe(id, PeerObservation{PeerID: id, MessageReceived: true, DeliverySuccess: true, MessageForwarded: true, Connected: true, PeerScore: 1})
	first := m.Score(id)
	if first <= .5 || first >= 1 {
		t.Fatalf("unexpected smoothed trust: %v", first)
	}
	m.Observe(id, PeerObservation{PeerID: id, Invalid: true, Connected: false})
	second := m.Score(id)
	if second <= 0 || second >= first {
		t.Fatalf("invalid observation should lower trust gradually: %v -> %v", first, second)
	}
	if math.IsNaN(second) || second < 0 || second > 1 {
		t.Fatalf("trust out of bounds: %v", second)
	}
}

func TestTrustManagerConcurrentUpdatesAndCopy(t *testing.T) {
	m := NewTrustManager(DefaultTrustWeights(), .25)
	id := peer.ID("peer-b")
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Observe(id, PeerObservation{PeerID: id, MessageReceived: true, Connected: true})
			}
		}()
	}
	wg.Wait()
	scores := m.AllScores()
	if scores[id].Observations != 1600 {
		t.Fatalf("observations = %d", scores[id].Observations)
	}
	delete(scores, id)
	if _, ok := m.Get(id); !ok {
		t.Fatal("AllScores returned mutable internal state")
	}
}

func TestTrustFeaturesRankingAndAggregation(t *testing.T) {
	a, b := peer.ID("a"), peer.ID("b")
	features := AggregateTrustFeatures(map[peer.ID]TrustScore{a: {PeerID: a, Score: .9}, b: {PeerID: b, Score: .3}}, .7)
	if features.AverageNeighborTrust != .6 || features.TrustedPeerRatio != .5 {
		t.Fatalf("unexpected features: %+v", features)
	}
	cfg := NewConfig(WithTrustScore(true), WithPeerScoreWeight(.25), WithTrustScoreWeight(.75))
	ranked := RankPeers([]peer.ID{b, a}, map[peer.ID]TrustScore{a: {Score: .9}, b: {Score: .2}}, map[peer.ID]float64{a: 0, b: 1}, cfg)
	if ranked[0] != a {
		t.Fatalf("trust should rank peer a first: %v", ranked)
	}
	agg := NewAggregator()
	merged, err := agg.TrustWeightedFedAvg([]ModelState{{Weights: []float64{0}, Biases: []float64{0}, Samples: 10}, {Weights: []float64{10}, Biases: []float64{10}, Samples: 10}}, []float64{1, 0}, 1)
	if err != nil || merged.Weights[0] >= 5 {
		t.Fatalf("unexpected trust aggregation: %+v, %v", merged, err)
	}
}

func TestTrustDisabledAndModelValidation(t *testing.T) {
	if DefaultConfig().EnableTrustScore {
		t.Fatal("trust must be opt-in")
	}
	if err := ValidateModelState(ModelState{Weights: []float64{math.NaN()}}, 0, 0); err == nil {
		t.Fatal("NaN model update accepted")
	}
	if err := ValidateModelState(ModelState{Weights: []float64{1}, FeatureVersion: 1}, TrustModelVersion, 0); err == nil {
		t.Fatal("incompatible model version accepted")
	}
}
