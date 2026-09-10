package implementations

import (
	"context"
	"fmt"
	"math"
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
	FLRoundTrace                                                                                             []FLRoundTrace
	// ModeledResourceCostTotal is a workload-scaled proxy (cost score ×
	// published messages), not Joules or a physical energy measurement.
	ModeledResourceCostTotal, ModeledResourceCostPerDeliveredMessage float64
}

// FLRoundTrace records the complete controlled causal path for one round:
// aggregation -> model output -> bounded candidate -> controlled outcome.
type FLRoundTrace struct {
	Round                      uint64
	Method                     string
	ModelOutput                []float64
	Current                    fedgreensub.GossipParameters
	Candidate                  fedgreensub.GossipParameters
	CurrentOutcome             fedgreensub.CandidateScore
	ControlledOutcome          fedgreensub.CandidateScore
	CurrentObjective           float64
	CandidateObjective         float64
	Accepted                   bool
	RejectionReason            string
	AggregationParameterChange float64
	PredictorParameterChange   float64
	DecodedMesh                float64
	MeshCandidates             []int
	MeshCandidateObjectives    map[int]float64
	EffectiveWeights           map[string]float64
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
			var err error
			predicted, err = a.runFederatedRounds(workload)
			if err != nil {
				return err
			}
		}
		if accepted, reason := deploymentDecision(parameters, predicted, a.profiles, workload); accepted {
			_ = a.params.ApplyParameters(predicted)
			parameters = predicted
		} else {
			// Keep the last known-good runtime configuration.
			parameters = parameters
			_ = reason
		}
	}
	if a.config.TrustAware {
		a.recordTrustMetrics()
	}
	outcome := aggregateOutcome(a.profiles, workload, parameters, parameters.MeshDegree)
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
	a.metrics.ModeledResourceCostTotal = outcome.EnergyCost * float64(workload.Messages)
	if a.config.FedGreen && a.coordinator == nil {
		a.metrics.TrainingLoss, a.metrics.GlobalLoss = 0, 0
		a.metrics.FLRounds = 0
	}
	// Preserve the controlled simulator's continuous expected outcomes in the
	// reported ratios. Integer message counters remain rounded counts, but must
	// not erase sub-message candidate effects before analysis.
	a.metrics.DeliveryRatio = outcome.DeliveryRatio
	a.metrics.DuplicateRatio = outcome.DuplicateRatio
	if a.metrics.Delivered > 0 {
		a.metrics.ModeledResourceCostPerDeliveredMessage = a.metrics.ModeledResourceCostTotal / float64(a.metrics.Delivered)
	}
	a.metrics.EnergyPerDeliveredMessage = a.metrics.ModeledResourceCostPerDeliveredMessage
	return nil
}

func (a *adapter) runFederatedRounds(workload Workload) (fedgreensub.GossipParameters, error) {
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
	// runStart is the immutable configuration that existed at the beginning of
	// this FL run. It is a fixed reference point for the run-start guardrail
	// below and must never be reassigned or mutated after this point.
	runStart := defaultParameters()
	for round := 0; round < maxInt(a.config.FLRounds, 0); round++ {
		for i, profile := range a.profiles {
			participantWorkload := workload
			participantWorkload.PacketLoss = clamp(workload.PacketLoss + profile.packetLossOffset)
			participantWorkload.Latency = workload.Latency + profile.latencyOffset
			participantWorkload.Duplicates = clamp(workload.Duplicates + profile.duplicateOffset)
			metrics := fedgreensub.RuntimeMetrics{PeerCount: workload.Peers, PacketLossRate: participantWorkload.PacketLoss, DuplicateRate: participantWorkload.Duplicates, PublishLatency: float64(participantWorkload.Latency.Milliseconds()), BandwidthInBps: profile.bandwidthBps, BandwidthOutBps: profile.bandwidthBps * .8, CPU: profile.cpu, MeshDegree: parameters.MeshDegree, HeartbeatDuration: parameters.HeartbeatInterval}
			sample, _, err := fedgreensub.GenerateTrainingSample(context.Background(), metrics, parameters, simulationCandidateEvaluator{profile: profile, workload: participantWorkload, currentMesh: parameters.MeshDegree}, cfg)
			if err == nil {
				_ = a.coordinator.AppendParticipantTrainingSample(fmt.Sprintf("participant-%d", i), sample)
			}
		}
		if result, err := a.coordinator.RunRound(context.Background(), method); err == nil {
			predictorBefore := a.learned.ModelState()
			if err := a.learned.ImportModelState(result.Global); err != nil {
				return parameters, fmt.Errorf("import federated global model into predictor: %w", err)
			}
			predictorAfter := a.learned.ModelState()
			a.metrics.FLRounds = result.Round
			a.metrics.TrainingLoss = result.AverageLocalLoss
			a.metrics.GlobalLoss = result.Global.Loss
			a.metrics.TrainingTime += durationMilliseconds(result.LocalTrainingTime)
			a.metrics.AggregationTime += durationMilliseconds(result.AggregationTime)
			a.metrics.TotalRoundTime += durationMilliseconds(result.LocalTrainingTime + result.AggregationTime)
			a.metrics.ParameterChange = result.ParameterChange
			a.metrics.Contributors = result.Contributors
			a.metrics.Rejected = result.Rejected
			a.metrics.EffectiveWeights = result.EffectiveWeights
			a.metrics.LocalLossByRound = append(a.metrics.LocalLossByRound, result.AverageLocalLoss)
			a.metrics.GlobalLossByRound = append(a.metrics.GlobalLossByRound, result.GlobalLoss)
			a.metrics.ParameterChangeByRound = append(a.metrics.ParameterChangeByRound, result.ParameterChange)
			a.metrics.TrainingTimeByRound = append(a.metrics.TrainingTimeByRound, durationMilliseconds(result.LocalTrainingTime))
			a.metrics.AggregationTimeByRound = append(a.metrics.AggregationTimeByRound, durationMilliseconds(result.AggregationTime))
			modelOutput, predictedParameters := a.learned.PredictWithOutput(metricsForWorkload(workload, parameters), parameters)
			nextParameters, decodedMesh, meshCandidates, meshObjectives := selectNeighborMesh(parameters, predictedParameters, modelOutput, a.profiles, workload, cfg)
			// currentOutcome/candidateOutcome (and the objectives derived from
			// them) are anchored to the actually deployed mesh degree
			// (parameters.MeshDegree), not to each candidate's own mesh, so the
			// candidate is judged against what is really running, not against
			// itself. These are also the values recorded in FLRoundTrace below,
			// unchanged from before: they describe the candidate-vs-previous
			// comparison specifically.
			currentOutcome, candidateOutcome, currentObjective, candidateObjective, acceptedVsPrevious, reasonVsPrevious := evaluateGuardrailReference(a.profiles, workload, cfg, parameters, nextParameters)
			// A candidate must pass the existing candidate-vs-previous guardrail
			// AND a candidate-vs-run-start guardrail before it is deployed. This
			// prevents a sequence of individually small, individually-accepted
			// per-round steps from cumulatively drifting the deployed
			// configuration far from the configuration that was actually known
			// safe at the start of this FL run. If the previous-round check
			// already fails there is no need to also evaluate run-start.
			accepted, reason := acceptedVsPrevious, "vs previous: "+reasonVsPrevious
			if acceptedVsPrevious {
				_, _, _, _, acceptedVsRunStart, reasonVsRunStart := evaluateGuardrailReference(a.profiles, workload, cfg, runStart, nextParameters)
				accepted, reason = acceptedVsRunStart, "vs run-start: "+reasonVsRunStart
			}
			a.metrics.FLRoundTrace = append(a.metrics.FLRoundTrace, FLRoundTrace{Round: result.Round, Method: string(result.Method), ModelOutput: append([]float64(nil), modelOutput...), Current: parameters, Candidate: nextParameters, CurrentOutcome: currentOutcome, ControlledOutcome: candidateOutcome, CurrentObjective: currentObjective, CandidateObjective: candidateObjective, Accepted: accepted, RejectionReason: reason, AggregationParameterChange: result.ParameterChange, PredictorParameterChange: modelStateDistance(predictorBefore, predictorAfter), DecodedMesh: decodedMesh, MeshCandidates: meshCandidates, MeshCandidateObjectives: meshObjectives, EffectiveWeights: cloneWeights(result.EffectiveWeights)})
			if accepted {
				parameters = nextParameters
			}
		}
	}
	return parameters, nil
}

func modelStateDistance(left, right fedgreensub.ModelState) float64 {
	sum := 0.0
	for i := range left.Weights {
		if i < len(right.Weights) {
			d := right.Weights[i] - left.Weights[i]
			sum += d * d
		}
	}
	for i := range left.Biases {
		if i < len(right.Biases) {
			d := right.Biases[i] - left.Biases[i]
			sum += d * d
		}
	}
	return math.Sqrt(sum)
}

func selectNeighborMesh(current, predicted fedgreensub.GossipParameters, output []float64, profiles []controlledParticipantProfile, workload Workload, cfg fedgreensub.Config) (fedgreensub.GossipParameters, float64, []int, map[int]float64) {
	decoded := float64(predicted.MeshDegree)
	if len(output) > 0 {
		decoded = fedgreensub.DecodedMeshValue(output[0], current.MeshDegree)
	}
	values := []int{int(math.Floor(decoded)), int(math.Round(decoded)), int(math.Ceil(decoded))}
	seen := make(map[int]struct{}, len(values))
	candidates := make([]int, 0, len(values))
	objectives := make(map[int]float64, len(values))
	for _, mesh := range values {
		mesh = clampMesh(mesh, cfg.MinMeshDegree, cfg.MaxMeshDegree)
		if _, ok := seen[mesh]; ok {
			continue
		}
		seen[mesh] = struct{}{}
		candidate := predicted
		candidate.MeshDegree = mesh
		candidate.DLow = clampMesh(mesh/2, cfg.MinMeshDegree, mesh)
		candidate.DHigh = clampMesh(mesh+4, mesh, cfg.MaxMeshDegree)
		// Every neighbor candidate is judged against the mesh degree actually
		// deployed today (current.MeshDegree), not against itself.
		outcome := aggregateOutcome(profiles, workload, candidate, current.MeshDegree)
		objective := fedgreensub.ScoreCandidate(outcome, cfg.MinimumDeliveryRatio, cfg.TargetWeights)
		candidates = append(candidates, mesh)
		objectives[mesh] = objective
	}
	bestMesh := selectBestMeshCandidate(candidates, objectives, predicted.MeshDegree, decoded)
	best := predicted
	best.MeshDegree = bestMesh
	best.DLow = clampMesh(bestMesh/2, cfg.MinMeshDegree, bestMesh)
	best.DHigh = clampMesh(bestMesh+4, bestMesh, cfg.MaxMeshDegree)
	return best, decoded, candidates, objectives
}

func clampMesh(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func selectBestMeshCandidate(candidates []int, objectives map[int]float64, fallback int, preferred ...float64) int {
	best, bestObjective := fallback, math.Inf(1)
	bestDistance := math.Inf(1)
	preferredValue := float64(fallback)
	if len(preferred) > 0 {
		preferredValue = preferred[0]
	}
	for _, mesh := range candidates {
		objective, ok := objectives[mesh]
		if !ok || math.IsNaN(objective) || math.IsInf(objective, 0) {
			continue
		}
		distance := math.Abs(float64(mesh) - preferredValue)
		if objective < bestObjective-1e-12 || (math.Abs(objective-bestObjective) <= 1e-12 && distance < bestDistance) {
			best, bestObjective = mesh, objective
			bestDistance = distance
		}
	}
	return best
}

// evaluateGuardrailReference scores candidate relative to reference using the
// existing deploymentDecisionWithScores machinery, without duplicating its
// tolerance/objective logic. Both outcomes are evaluated with reference's own
// mesh degree as the evaluator's redundancy baseline (the same convention
// deploymentDecision and selectNeighborMesh already use), so this can be
// called with either the previously deployed configuration or the immutable
// run-start configuration as reference to ask "is candidate safe relative to
// this baseline?"
func evaluateGuardrailReference(profiles []controlledParticipantProfile, workload Workload, cfg fedgreensub.Config, reference, candidate fedgreensub.GossipParameters) (referenceOutcome, candidateOutcome fedgreensub.CandidateScore, referenceObjective, candidateObjective float64, accepted bool, reason string) {
	referenceOutcome = aggregateOutcome(profiles, workload, reference, reference.MeshDegree)
	candidateOutcome = aggregateOutcome(profiles, workload, candidate, reference.MeshDegree)
	referenceObjective = fedgreensub.ScoreCandidate(referenceOutcome, cfg.MinimumDeliveryRatio, cfg.TargetWeights)
	candidateObjective = fedgreensub.ScoreCandidate(candidateOutcome, cfg.MinimumDeliveryRatio, cfg.TargetWeights)
	accepted, reason = deploymentDecisionWithScores(referenceOutcome, candidateOutcome, referenceObjective, candidateObjective)
	return
}

// deploymentDecision compares a learned update with the current runtime state.
// The objective remains primary; hard tolerances prevent severe regressions in
// delivery, duplicate traffic, or modeled resource cost. This is the outer,
// whole-run guardrail called once in run() after all FL rounds complete; it
// remains unchanged and serves as a final belt-and-suspenders check on top of
// the per-round dual guardrail in runFederatedRounds.
func deploymentDecision(current, candidate fedgreensub.GossipParameters, profiles []controlledParticipantProfile, workload Workload) (bool, string) {
	currentOutcome := aggregateOutcome(profiles, workload, current, current.MeshDegree)
	candidateOutcome := aggregateOutcome(profiles, workload, candidate, current.MeshDegree)
	cfg := fedgreensub.DefaultConfig()
	return deploymentDecisionWithScores(currentOutcome, candidateOutcome, fedgreensub.ScoreCandidate(currentOutcome, cfg.MinimumDeliveryRatio, cfg.TargetWeights), fedgreensub.ScoreCandidate(candidateOutcome, cfg.MinimumDeliveryRatio, cfg.TargetWeights))
}

func deploymentDecisionWithScores(current, candidate fedgreensub.CandidateScore, currentObjective, candidateObjective float64) (bool, string) {
	const deliveryTolerance = .01
	const duplicateTolerance = .02
	const resourceTolerance = .02
	if !candidate.Valid {
		return false, "candidate outcome invalid"
	}
	if candidate.DeliveryRatio < current.DeliveryRatio-deliveryTolerance {
		return false, "delivery regression exceeds tolerance"
	}
	if candidate.DuplicateRatio > current.DuplicateRatio+duplicateTolerance {
		return false, "duplicate regression exceeds tolerance"
	}
	if candidate.EnergyCost > current.EnergyCost+resourceTolerance {
		return false, "modeled resource-cost regression exceeds tolerance"
	}
	if candidateObjective > currentObjective+1e-9 {
		return false, "objective is worse than current configuration"
	}
	return true, "accepted: objective improved or remained equal within tolerances"
}

func durationMilliseconds(duration time.Duration) float64 {
	// Windows timer resolution can report an extremely short operation as
	// zero. Keep the value in milliseconds while retaining a 1ns lower bound;
	// this marks "below timer resolution" instead of silently exporting zero.
	if duration <= 0 {
		return 0.000001
	}
	return float64(duration) / float64(time.Millisecond)
}

func cloneWeights(weights map[string]float64) map[string]float64 {
	if weights == nil {
		return nil
	}
	clone := make(map[string]float64, len(weights))
	for id, weight := range weights {
		clone[id] = weight
	}
	return clone
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
	// currentMesh is the actually deployed mesh degree, set explicitly by the
	// caller rather than inferred from RuntimeMetrics, so target generation
	// never silently depends on a metrics field happening to match the real
	// deployed configuration.
	currentMesh int
}

func (e simulationCandidateEvaluator) Evaluate(_ context.Context, _ fedgreensub.RuntimeMetrics, candidate fedgreensub.GossipParameters) (fedgreensub.CandidateScore, error) {
	return controlledOutcome(e.profile, e.workload, candidate, e.currentMesh), nil
}

// controlledOutcome is a deterministic, synthetic stand-in for a real network
// measurement. It is intentionally simple, and the mesh-degree tradeoff it
// encodes is documented here so it can be checked against these comments
// rather than reverse-engineered from magic numbers.
//
// Mesh degree affects the outcome through two independent, bounded effects:
//
//  1. Absolute forwarding overhead (duplicateDensity, latencyDensityMs): a
//     wider mesh forwards more redundant copies of every message and costs
//     more per-heartbeat connection bookkeeping, regardless of what is
//     currently deployed. This is monotonically increasing in the
//     candidate's own mesh degree, so mesh 6, 8, and 10 are never
//     numerically identical on this axis (unlike the previous formulation,
//     which only reacted to mesh degrees above a hardcoded value of 8 and
//     therefore made every mesh degree <= 8 objectively indistinguishable).
//
//  2. Redundancy lost relative to the currently deployed mesh (deliveryRisk,
//     latencyDeficitMs): a gossip mesh tolerates lost or failed links
//     because messages have alternative forwarding paths. Deploying a
//     sparser mesh than what is running today removes some of those paths.
//     The resulting delivery/latency risk is amplified by how lossy the
//     link already is (lossSeverity): a healthy link barely notices losing
//     a path, an already-lossy one notices more. currentMesh is the mesh
//     degree actually running today, not a hardcoded constant, so this term
//     is anchored to the real deployed reference point.
//
// Neither effect dominates the objective on its own; they exist to make
// mesh degree a genuine, separable input instead of a value that only
// matters through gossip factor and heartbeat. Whether a lower or higher
// mesh degree ultimately scores better is left to fall out of these two
// effects plus the existing delivery-threshold penalty in ScoreCandidate —
// it is not hardcoded here.
func controlledOutcome(profile controlledParticipantProfile, workload Workload, candidate fedgreensub.GossipParameters, currentMesh int) fedgreensub.CandidateScore {
	baseLoss := clamp(workload.PacketLoss + profile.packetLossOffset)
	lossSeverity := 1 + baseLoss*4 // 1.0 at zero loss, ~2.0 at 25% loss

	meshDeficit := float64(maxInt(currentMesh-candidate.MeshDegree, 0))
	duplicateDensity := float64(candidate.MeshDegree) * .005
	latencyDensityMs := float64(candidate.MeshDegree) * .8
	deliveryRisk := meshDeficit * .006 * lossSeverity
	latencyDeficitMs := meshDeficit * 3 * lossSeverity

	delivery := clamp(1 - baseLoss - deliveryRisk)
	duplicate := clamp(workload.Duplicates + profile.duplicateOffset + candidate.GossipFactor*.12 + duplicateDensity)
	latencyMs := float64(workload.Latency.Milliseconds()) + float64(profile.latencyOffset.Milliseconds()) + candidate.HeartbeatInterval.Seconds()*15 + latencyDensityMs + latencyDeficitMs
	if latencyMs < 0 {
		latencyMs = 0
	}
	// Use the package's single normalized modeled-resource estimator for every
	// configuration. Memory is unavailable in this controlled profile and is
	// therefore left at zero; this is not a physical energy measurement.
	resourceMetrics := fedgreensub.RuntimeMetrics{
		CPU:               profile.cpu,
		BandwidthInBps:    profile.bandwidthBps,
		BandwidthOutBps:   profile.bandwidthBps * .8,
		DuplicateRate:     duplicate,
		HeartbeatDuration: candidate.HeartbeatInterval,
	}
	resource := fedgreensub.EstimateEnergy(resourceMetrics)
	return fedgreensub.CandidateScore{Parameters: candidate, DeliveryRatio: delivery, DuplicateRatio: duplicate, LatencyCost: clamp(latencyMs / 1000), EnergyCost: resource, Valid: true}
}

// aggregateOutcome evaluates candidate against the controlled participant
// profiles. currentMesh is the actually deployed mesh degree used as the
// reference point for the redundancy-lost effect in controlledOutcome; it is
// intentionally a separate argument from candidate so callers evaluating a
// candidate that differs from what is deployed do not accidentally treat the
// candidate as its own reference.
func aggregateOutcome(profiles []controlledParticipantProfile, workload Workload, candidate fedgreensub.GossipParameters, currentMesh int) fedgreensub.CandidateScore {
	if len(profiles) == 0 {
		return controlledOutcome(controlledProfile(0, workload.RandomSeed), workload, candidate, currentMesh)
	}
	result := fedgreensub.CandidateScore{Parameters: candidate, Valid: true}
	for _, profile := range profiles {
		outcome := controlledOutcome(profile, workload, candidate, currentMesh)
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

// DefaultParameters exposes the package's no-adaptation baseline
// configuration for read-only reporting/export use (e.g. distinguishing "no
// FL round was ever accepted" from an actually-adapted deployed
// configuration). It does not change evaluator or benchmark behavior.
func DefaultParameters() fedgreensub.GossipParameters {
	return defaultParameters()
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
