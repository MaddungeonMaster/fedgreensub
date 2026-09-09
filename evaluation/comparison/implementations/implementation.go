package implementations

import (
	"context"
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
	Heuristic      bool
	Aggregation    fedgreensub.AggregationMethod
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
	TotalRoundTime, ParameterChange                                                                          float64
	Contributors, Rejected                                                                                   int
	LocalLossByRound, GlobalLossByRound, ParameterChangeByRound                                              []float64
	TrainingTimeByRound, AggregationTimeByRound                                                              []float64
	EffectiveWeights                                                                                         map[string]float64
}

type Implementation interface {
	Name() string
	Start(Config) error
	RunWorkload(Workload) error
	Stop() error
	Metrics() Metrics
}

type adapter struct {
	config      Config
	metrics     Metrics
	random      *rand.Rand
	optimizer   *fedgreensub.Optimizer
	params      *fedgreensub.ParameterManager
	coordinator *fedgreensub.FederatedCoordinator
	learned     *fedgreensub.LearnedPredictor
	profiles    []controlledParticipantProfile
}

func (a *adapter) start(config Config) {
	a.config = config
	a.metrics = Metrics{}
	a.profiles = nil
	a.random = rand.New(rand.NewSource(1))
	if config.FedGreen {
		cfg := fedgreensub.NewConfig(fedgreensub.WithAdaptiveMode(true), fedgreensub.WithFederatedLearning(true), fedgreensub.WithTrustScore(config.TrustAware))
		// FL targets and predictions are candidate-evaluated/learned. The
		// heuristic predictor is deliberately not constructed on this path.
		a.optimizer = fedgreensub.NewOptimizer(cfg, fedgreensub.NewCollector(), nil, nil)
		a.params = fedgreensub.NewParameterManager(cfg)
		if !config.Heuristic {
			network, err := fedgreensub.NewTinyNetwork(fedgreensub.ModelInputSize, 8, fedgreensub.ModelOutputSize, cfg.LearningRate)
			if err == nil {
				a.learned = fedgreensub.NewLearnedPredictor(network, cfg)
				a.coordinator, _ = fedgreensub.NewFederatedCoordinator(network.Weights(), cfg.ModelUpdateClipping)
			}
		}
	}
}

func (a *adapter) run(workload Workload) error {
	if workload.Peers <= 0 || workload.Messages < 0 {
		return nil
	}
	a.profiles = make([]controlledParticipantProfile, workload.Peers)
	for i := range a.profiles {
		a.profiles[i] = controlledProfile(i, workload.RandomSeed)
	}
	parameters := defaultParameters()
	a.metrics.HeartbeatCount = uint64(workload.Duration / time.Second)
	if a.config.FedGreen {
		predicted := parameters
		if a.config.Heuristic {
			predicted = a.optimizerPrediction(workload)
		} else if a.coordinator != nil && a.learned != nil {
			predicted = a.runFederatedRounds(workload)
		}
		a.params.ApplyParameters(predicted)
		parameters = predicted
	}
	if a.config.TrustAware {
		a.recordTrustMetrics()
	}
	outcome := aggregateOutcome(a.profiles, workload, parameters)
	a.metrics.Published = uint64(workload.Messages)
	a.metrics.Delivered = uint64(float64(workload.Messages) * outcome.DeliveryRatio)
	a.metrics.Received = a.metrics.Delivered + uint64(float64(workload.Messages)*outcome.DuplicateRatio)
	a.metrics.Duplicates = a.metrics.Received - a.metrics.Delivered
	baseBytes := float64(workload.Messages * workload.MessageSize)
	a.metrics.BytesSent = uint64(baseBytes * (1 + outcome.DuplicateRatio))
	a.metrics.BytesReceived = uint64(float64(a.metrics.BytesSent) * outcome.DeliveryRatio)
	a.metrics.LatencyAverage = outcome.LatencyCost * 1000
	a.metrics.LatencyP95 = a.metrics.LatencyAverage * 1.15
	a.metrics.MeshDegree = float64(parameters.MeshDegree)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	metrics := fedgreensub.RuntimeMetrics{MemoryMB: float64(mem.Alloc) / (1024 * 1024), BandwidthInBps: float64(a.metrics.BytesSent) / workload.Duration.Seconds(), BandwidthOutBps: float64(a.metrics.BytesReceived) / workload.Duration.Seconds(), DuplicateRate: outcome.DuplicateRatio, MeshDegree: int(a.metrics.MeshDegree), PeerCount: workload.Peers, PublishLatency: a.metrics.LatencyAverage, HeartbeatDuration: parameters.HeartbeatInterval, CPU: averageCPU(a.profiles)}
	a.metrics.CPU, a.metrics.MemoryMB = metrics.CPU, metrics.MemoryMB
	a.metrics.Energy = outcome.EnergyCost
	if a.config.FedGreen && a.coordinator == nil {
		a.metrics.TrainingLoss, a.metrics.GlobalLoss = 0, 0
		a.metrics.FLRounds = 0
	}
	a.metrics.DeliveryRatio = ratio(a.metrics.Delivered, a.metrics.Published)
	a.metrics.DuplicateRatio = ratio(a.metrics.Duplicates, a.metrics.Received)
	a.metrics.EnergyPerDeliveredMessage = ratioFloat(a.metrics.Energy, a.metrics.Delivered)
	return nil
}

func (a *adapter) runFederatedRounds(workload Workload) fedgreensub.GossipParameters {
	cfg := fedgreensub.DefaultConfig()
	for i, profile := range a.profiles {
		local, err := fedgreensub.NewTinyNetwork(fedgreensub.ModelInputSize, 8, fedgreensub.ModelOutputSize, cfg.LearningRate)
		if err != nil {
			continue
		}
		trust := .5
		if a.config.TrustAware {
			success := i%3 != 0
			a.optimizer.ObservePeer(peerID(i), fedgreensub.PeerObservation{PeerID: peerID(i), MessageReceived: success, DeliverySuccess: success, MessageForwarded: success, Connected: success, PacketLoss: workload.PacketLoss, Latency: workload.Latency})
			trust = a.optimizer.TrustManager().Score(peerID(i))
		}
		_ = a.coordinator.Register(&fedgreensub.FederatedParticipant{ID: fmt.Sprintf("participant-%d", i), Trainer: fedgreensub.NewNetworkTrainer(local, cfg.LocalTrainingEpochs), ResourceWeight: profile.resourceAvailability, TrustScore: trust, TrustKnown: a.config.TrustAware})
	}
	method := a.config.Aggregation
	if method == "" {
		method = fedgreensub.EnergyFedAvgMethod
	}
	if a.config.TrustAware {
		method = fedgreensub.TrustFedAvgMethod
	}
	parameters := defaultParameters()
	for round := 0; round < maxInt(a.config.FLRounds, 0); round++ {
		for i, profile := range a.profiles {
			participantWorkload := workload
			participantWorkload.PacketLoss = clamp(workload.PacketLoss + profile.packetLossOffset)
			participantWorkload.Latency = workload.Latency + profile.latencyOffset
			participantWorkload.Duplicates = clamp(workload.Duplicates + profile.duplicateOffset)
			metrics := fedgreensub.RuntimeMetrics{PeerCount: workload.Peers, PacketLossRate: participantWorkload.PacketLoss, DuplicateRate: participantWorkload.Duplicates, PublishLatency: float64(participantWorkload.Latency.Milliseconds()), BandwidthInBps: profile.bandwidthBps, BandwidthOutBps: profile.bandwidthBps * .8, CPU: profile.cpu, MeshDegree: parameters.MeshDegree, HeartbeatDuration: parameters.HeartbeatInterval}
			sample, _, err := fedgreensub.GenerateTrainingSample(context.Background(), metrics, parameters, simulationCandidateEvaluator{profile: profile, workload: participantWorkload}, cfg)
			if err == nil {
				_ = a.coordinator.AppendParticipantTrainingSample(fmt.Sprintf("participant-%d", i), sample)
			}
		}
		if result, err := a.coordinator.RunRound(context.Background(), method); err == nil {
			a.metrics.FLRounds = result.Round
			a.metrics.TrainingLoss = result.AverageLocalLoss
			a.metrics.GlobalLoss = result.Global.Loss
			a.metrics.TrainingTime += float64(result.LocalTrainingTime.Nanoseconds())
			a.metrics.AggregationTime += float64(result.AggregationTime.Nanoseconds())
			a.metrics.TotalRoundTime += float64(result.LocalTrainingTime.Nanoseconds() + result.AggregationTime.Nanoseconds())
			a.metrics.ParameterChange = result.ParameterChange
			a.metrics.Contributors = result.Contributors
			a.metrics.Rejected = result.Rejected
			a.metrics.EffectiveWeights = result.EffectiveWeights
			a.metrics.LocalLossByRound = append(a.metrics.LocalLossByRound, result.AverageLocalLoss)
			a.metrics.GlobalLossByRound = append(a.metrics.GlobalLossByRound, result.GlobalLoss)
			a.metrics.ParameterChangeByRound = append(a.metrics.ParameterChangeByRound, result.ParameterChange)
			a.metrics.TrainingTimeByRound = append(a.metrics.TrainingTimeByRound, float64(result.LocalTrainingTime.Nanoseconds()))
			a.metrics.AggregationTimeByRound = append(a.metrics.AggregationTimeByRound, float64(result.AggregationTime.Nanoseconds()))
			parameters = a.learned.Predict(metricsForWorkload(workload, parameters))
		}
	}
	return parameters
}

func (a *adapter) recordTrustMetrics() {
	if a.optimizer == nil || a.optimizer.TrustManager() == nil {
		return
	}
	scores := a.optimizer.TrustManager().AllScores()
	features := fedgreensub.AggregateTrustFeatures(scores, .7)
	a.metrics.AverageTrust = features.AverageNeighborTrust
	a.metrics.MinimumTrust = features.MinimumNeighborTrust
	a.metrics.TrustVariance = features.TrustVariance
	a.metrics.TrustedPeerRatio = features.TrustedPeerRatio
}

// controlledProfile is deterministic testbed metadata, not a live network or
// physical energy observation. It creates meaningfully non-IID local data.
type controlledParticipantProfile struct {
	packetLossOffset                                         float64
	latencyOffset                                            time.Duration
	duplicateOffset, bandwidthBps, cpu, resourceAvailability float64
}

func controlledProfile(i int, seed int64) controlledParticipantProfile {
	profiles := []controlledParticipantProfile{
		{-.05, -15 * time.Millisecond, -.01, 20e6, 20, 1.0},
		{.00, 0, .00, 8e6, 45, .75},
		{.10, 40 * time.Millisecond, .05, 2e6, 75, .45},
		{.20, 90 * time.Millisecond, .10, 1e6, 90, .25},
	}
	profile := profiles[i%len(profiles)]
	random := rand.New(rand.NewSource(seed + int64(i+1)*7919))
	profile.packetLossOffset += (random.Float64() - .5) * .04
	profile.latencyOffset += time.Duration((random.Float64()-.5)*20) * time.Millisecond
	profile.duplicateOffset += (random.Float64() - .5) * .02
	profile.bandwidthBps *= .9 + random.Float64()*.2
	profile.cpu = clampRange(profile.cpu+(random.Float64()-.5)*10, 0, 100)
	profile.resourceAvailability = clamp(profile.resourceAvailability + (random.Float64()-.5)*.1)
	return profile
}

type simulationCandidateEvaluator struct {
	profile  controlledParticipantProfile
	workload Workload
}

func (e simulationCandidateEvaluator) Evaluate(_ context.Context, _ fedgreensub.RuntimeMetrics, candidate fedgreensub.GossipParameters) (fedgreensub.CandidateScore, error) {
	return controlledOutcome(e.profile, e.workload, candidate), nil
}

func controlledOutcome(profile controlledParticipantProfile, workload Workload, candidate fedgreensub.GossipParameters) fedgreensub.CandidateScore {
	meshPressure := float64(maxInt(candidate.MeshDegree-8, 0)) / 16
	meshDeficit := float64(maxInt(5-candidate.MeshDegree, 0)) / 5
	baseLoss := clamp(workload.PacketLoss + profile.packetLossOffset)
	delivery := clamp(1 - baseLoss - meshDeficit*.08 + meshPressure*.01)
	duplicate := clamp(workload.Duplicates + profile.duplicateOffset + candidate.GossipFactor*.12 + meshPressure*.04)
	latencyMs := float64(workload.Latency.Milliseconds()) + float64(profile.latencyOffset.Milliseconds()) + candidate.HeartbeatInterval.Seconds()*15 + meshPressure*20
	if latencyMs < 0 {
		latencyMs = 0
	}
	bandwidthCost := clamp((profile.bandwidthBps + profile.bandwidthBps*.8) / (40 * 1024 * 1024))
	resource := clamp(.35*profile.cpu/100 + .25*bandwidthCost + .15*float64(candidate.MeshDegree)/16 + .15*duplicate + .10*candidate.HeartbeatInterval.Seconds()/10)
	return fedgreensub.CandidateScore{Parameters: candidate, DeliveryRatio: delivery, DuplicateRatio: duplicate, LatencyCost: clamp(latencyMs / 1000), EnergyCost: resource, Valid: true}
}

func aggregateOutcome(profiles []controlledParticipantProfile, workload Workload, candidate fedgreensub.GossipParameters) fedgreensub.CandidateScore {
	if len(profiles) == 0 {
		return controlledOutcome(controlledProfile(0, workload.RandomSeed), workload, candidate)
	}
	result := fedgreensub.CandidateScore{Parameters: candidate, Valid: true}
	for _, profile := range profiles {
		outcome := controlledOutcome(profile, workload, candidate)
		result.DeliveryRatio += outcome.DeliveryRatio
		result.DuplicateRatio += outcome.DuplicateRatio
		result.LatencyCost += outcome.LatencyCost
		result.EnergyCost += outcome.EnergyCost
	}
	count := float64(len(profiles))
	result.DeliveryRatio /= count
	result.DuplicateRatio /= count
	result.LatencyCost /= count
	result.EnergyCost /= count
	return result
}

func averageCPU(profiles []controlledParticipantProfile) float64 {
	if len(profiles) == 0 {
		return 0
	}
	total := 0.0
	for _, profile := range profiles {
		total += profile.cpu
	}
	return total / float64(len(profiles))
}

func defaultParameters() fedgreensub.GossipParameters {
	return fedgreensub.GossipParameters{MeshDegree: 8, DLow: 4, DHigh: 12, GossipFactor: .25, HeartbeatInterval: time.Second}
}

func metricsForWorkload(workload Workload, parameters fedgreensub.GossipParameters) fedgreensub.RuntimeMetrics {
	return fedgreensub.RuntimeMetrics{PeerCount: workload.Peers, PacketLossRate: workload.PacketLoss, DuplicateRate: workload.Duplicates, PublishLatency: float64(workload.Latency.Milliseconds()), MeshDegree: parameters.MeshDegree, HeartbeatDuration: parameters.HeartbeatInterval}
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func clampRange(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
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
