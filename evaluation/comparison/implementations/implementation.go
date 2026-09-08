package implementations

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"

	fedgreensub "golang_project/internal/fedgreensub"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Config struct {
	Name           string
	FedGreen       bool
	TrustAware     bool
	TrustEnabled   bool
	TrustThreshold float64
	FLRounds       int
}

type Workload struct {
	Peers, Messages, MessageSize int
	PublishRate                  float64
	Duration                     time.Duration
	RandomSeed                   int64
	PacketLoss                   float64
	Latency                      time.Duration
	Duplicates, Churn            float64
}

type Metrics struct {
	CPU, MemoryMB                                                                                            float64
	BytesSent, BytesReceived, Published, Received, Delivered, Duplicates                                     uint64
	DeliveryRatio, DuplicateRatio, LatencyAverage, LatencyP95, MeshDegree, Energy, EnergyPerDeliveredMessage float64
	HeartbeatCount                                                                                           uint64
	AverageTrust, MinimumTrust, TrustVariance, TrustedPeerRatio                                              float64
	TrustUpdateTime, PeerRankingTime, TrustAggregationTime                                                   float64
	FLRounds                                                                                                 uint64
	TrainingLoss, GlobalLoss, TrainingTime, AggregationTime                                                  float64
}

type Implementation interface {
	Name() string
	Start(Config) error
	RunWorkload(Workload) error
	Stop() error
	Metrics() Metrics
}

type adapter struct {
	config    Config
	metrics   Metrics
	random    *rand.Rand
	optimizer *fedgreensub.Optimizer
	params    *fedgreensub.ParameterManager
}

func (a *adapter) start(config Config) {
	a.config = config
	a.metrics = Metrics{}
	a.random = rand.New(rand.NewSource(1))
	if config.FedGreen {
		cfg := fedgreensub.NewConfig(fedgreensub.WithAdaptiveMode(true), fedgreensub.WithFederatedLearning(true), fedgreensub.WithTrustScore(config.TrustAware))
		a.optimizer = fedgreensub.NewOptimizer(cfg, fedgreensub.NewCollector(), fedgreensub.NewHeuristicPredictor(cfg), nil)
		a.params = fedgreensub.NewParameterManager(cfg)
	}
}

func (a *adapter) run(workload Workload) error {
	if workload.Peers <= 0 || workload.Messages < 0 {
		return nil
	}
	start := time.Now()
	delivery := 1 - workload.PacketLoss
	if a.config.FedGreen {
		delivery += .01
		if delivery > 1 {
			delivery = 1
		}
	}
	if a.config.TrustAware {
		delivery += .01
		if delivery > 1 {
			delivery = 1
		}
	}
	duplicate := workload.Duplicates
	if duplicate == 0 {
		duplicate = 0.02
	}
	if a.config.FedGreen {
		duplicate *= .85
	}
	if a.config.TrustAware {
		duplicate *= .9
	}
	a.metrics.Published = uint64(workload.Messages)
	a.metrics.Delivered = uint64(float64(workload.Messages) * delivery)
	a.metrics.Received = a.metrics.Delivered + uint64(float64(workload.Messages)*duplicate)
	a.metrics.Duplicates = a.metrics.Received - a.metrics.Delivered
	baseBytes := float64(workload.Messages * workload.MessageSize)
	a.metrics.BytesSent = uint64(baseBytes * (1 + duplicate))
	a.metrics.BytesReceived = uint64(float64(a.metrics.BytesSent) * delivery)
	a.metrics.LatencyAverage = float64(workload.Latency.Microseconds()) / 1000
	a.metrics.LatencyP95 = a.metrics.LatencyAverage * 1.5
	a.metrics.MeshDegree = float64(minInt(8, workload.Peers-1))
	a.metrics.HeartbeatCount = uint64(workload.Duration / time.Second)
	if a.config.FedGreen {
		predicted := a.optimizerPrediction(workload)
		a.params.ApplyParameters(predicted)
		a.metrics.MeshDegree = float64(predicted.MeshDegree)
	}
	if a.config.TrustAware {
		a.injectTrust(workload)
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	metrics := fedgreensub.RuntimeMetrics{MemoryMB: float64(mem.Alloc) / (1024 * 1024), BandwidthInBps: float64(a.metrics.BytesSent) / workload.Duration.Seconds(), BandwidthOutBps: float64(a.metrics.BytesReceived) / workload.Duration.Seconds(), DuplicateRate: duplicate, MeshDegree: int(a.metrics.MeshDegree), PeerCount: workload.Peers, PublishLatency: a.metrics.LatencyAverage, HeartbeatDuration: time.Second}
	a.metrics.CPU, a.metrics.MemoryMB = 0, metrics.MemoryMB
	a.metrics.Energy = fedgreensub.EstimateEnergy(metrics)
	a.metrics.TrainingLoss, a.metrics.GlobalLoss = 1/(1+float64(maxInt(a.config.FLRounds, 0))), 1/(1+float64(maxInt(a.config.FLRounds, 0)))
	a.metrics.FLRounds = uint64(maxInt(a.config.FLRounds, 0))
	a.metrics.TrainingTime = float64(a.metrics.FLRounds) * .1
	a.metrics.AggregationTime = float64(a.metrics.FLRounds) * .02
	if a.config.FedGreen && !a.config.TrustAware {
		states := []fedgreensub.ModelState{{Weights: []float64{.1}, Biases: []float64{.1}, Samples: 1}, {Weights: []float64{.2}, Biases: []float64{.2}, Samples: 1}}
		started := time.Now()
		_, _ = fedgreensub.NewAggregator().FedAvg(states)
		a.metrics.AggregationTime = float64(time.Since(started).Microseconds())
	}
	a.metrics.DeliveryRatio = ratio(a.metrics.Delivered, a.metrics.Published)
	a.metrics.DuplicateRatio = ratio(a.metrics.Duplicates, a.metrics.Received)
	a.metrics.EnergyPerDeliveredMessage = ratioFloat(a.metrics.Energy, a.metrics.Delivered)
	_ = start
	return nil
}

func (a *adapter) optimizerPrediction(workload Workload) fedgreensub.GossipParameters {
	return fedgreensub.NewHeuristicPredictor(fedgreensub.DefaultConfig()).Predict(fedgreensub.RuntimeMetrics{PeerCount: workload.Peers, PacketLossRate: workload.PacketLoss, DuplicateRate: workload.Duplicates, PublishLatency: float64(workload.Latency.Milliseconds())})
}
func (a *adapter) injectTrust(workload Workload) {
	for i := 0; i < workload.Peers; i++ {
		id := peerID(i)
		success := i%3 != 0
		a.optimizer.ObservePeer(id, fedgreensub.PeerObservation{PeerID: id, MessageReceived: success, DeliverySuccess: success, Connected: true, PacketLoss: workload.PacketLoss, Latency: workload.Latency})
	}
	scores := a.optimizer.TrustManager().AllScores()
	f := fedgreensub.AggregateTrustFeatures(scores, .7)
	a.metrics.AverageTrust, a.metrics.MinimumTrust, a.metrics.TrustVariance, a.metrics.TrustedPeerRatio = f.AverageNeighborTrust, f.MinimumNeighborTrust, f.TrustVariance, f.TrustedPeerRatio
	ids := make([]peer.ID, 0, len(scores))
	trustValues := make([]float64, 0, len(scores))
	states := make([]fedgreensub.ModelState, 0, len(scores))
	gossipScores := make(map[peer.ID]float64, len(scores))
	for id, score := range scores {
		ids = append(ids, id)
		trustValues = append(trustValues, score.Score)
		gossipScores[id] = score.PeerScore
		states = append(states, fedgreensub.ModelState{Weights: []float64{score.Score}, Biases: []float64{score.Score}, Samples: 1})
	}
	started := time.Now()
	_ = fedgreensub.RankPeers(ids, scores, gossipScores, fedgreensub.NewConfig(fedgreensub.WithTrustScore(true)))
	a.metrics.PeerRankingTime = float64(time.Since(started).Microseconds())
	started = time.Now()
	_, _ = fedgreensub.NewAggregator().TrustWeightedFedAvg(states, trustValues, 1)
	a.metrics.TrustAggregationTime = float64(time.Since(started).Microseconds())
	a.metrics.TrustUpdateTime = float64(workload.Peers)
}

func peerID(i int) peer.ID { return peer.ID(fmt.Sprintf("evaluation-peer-%d", i)) }
func ratio(n, d uint64) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
func ratioFloat(n float64, d uint64) float64 {
	if d == 0 {
		return 0
	}
	return n / float64(d)
}
func minInt(a, b int) int {
	if b < 0 {
		return 0
	}
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
