package fedgreensub

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

type AggregationMethod string

const (
	FedAvgMethod       AggregationMethod = "fedavg"
	EnergyFedAvgMethod AggregationMethod = "energy"
	TrustFedAvgMethod  AggregationMethod = "trust"
)

type FederatedParticipant struct {
	ID      string
	Trainer Trainer
	Dataset TrainingDataset
	// DatasetWindow holds runtime-generated candidate-outcome samples. When set,
	// it is the source of local training data for this participant.
	DatasetWindow *DatasetWindow
	// ResourceWeight is a normalized, modelled resource availability [0,1]. It
	// is not a physical energy measurement.
	ResourceWeight float64
	TrustScore     float64
	// TrustKnown distinguishes a newly registered participant (neutral trust)
	// from an observed participant whose score may legitimately be zero.
	TrustKnown bool
}

type FederatedRoundResult struct {
	Round             uint64
	Method            AggregationMethod
	Global            ModelState
	Contributors      int
	Rejected          int
	AverageLocalLoss  float64
	ParameterChange   float64
	LocalTrainingTime time.Duration
	AggregationTime   time.Duration
	// GlobalLoss is the aggregate mean local training loss; it is not a live
	// network delivery metric.
	GlobalLoss       float64
	EffectiveWeights map[string]float64
}

type FederatedCoordinator struct {
	mu       sync.RWMutex
	agg      Aggregator
	clipping float64
	global   ModelState
	round    uint64
	peers    map[string]*FederatedParticipant
}

func NewFederatedCoordinator(initial ModelState, clipping float64) (*FederatedCoordinator, error) {
	if err := ValidateModelState(initial, 0, clipping); err != nil {
		return nil, err
	}
	if clipping <= 0 {
		clipping = 10
	}
	return &FederatedCoordinator{agg: NewAggregator(), clipping: clipping, global: cloneModelState(initial), peers: make(map[string]*FederatedParticipant)}, nil
}

func (c *FederatedCoordinator) Register(participant *FederatedParticipant) error {
	if c == nil || participant == nil || participant.ID == "" || participant.Trainer == nil {
		return errors.New("fedgreensub: invalid federated participant")
	}
	c.mu.Lock()
	if participant.DatasetWindow == nil {
		participant.DatasetWindow = NewDatasetWindow(0)
		for _, sample := range participant.Dataset.Samples {
			participant.DatasetWindow.Append(sample)
		}
	}
	c.peers[participant.ID] = participant
	c.mu.Unlock()
	return nil
}

// AppendTrainingSample records one candidate-evaluated runtime observation in
// every controlled participant's local window. It deliberately does not create
// labels from the heuristic predictor.
func (c *FederatedCoordinator) AppendTrainingSample(sample TrainingSample) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, participant := range c.peers {
		if participant.DatasetWindow == nil {
			participant.DatasetWindow = NewDatasetWindow(0)
		}
		participant.DatasetWindow.Append(sample)
	}
}

// AppendParticipantTrainingSample records an observation for one local node.
func (c *FederatedCoordinator) AppendParticipantTrainingSample(id string, sample TrainingSample) error {
	if c == nil {
		return errors.New("fedgreensub: nil coordinator")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	participant, ok := c.peers[id]
	if !ok {
		return fmt.Errorf("fedgreensub: unknown participant %q", id)
	}
	if participant.DatasetWindow == nil {
		participant.DatasetWindow = NewDatasetWindow(0)
	}
	participant.DatasetWindow.Append(sample)
	return nil
}

func (c *FederatedCoordinator) GlobalModel() ModelState {
	if c == nil {
		return ModelState{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneModelState(c.global)
}

func (c *FederatedCoordinator) RunRound(ctx context.Context, method AggregationMethod) (FederatedRoundResult, error) {
	if c == nil {
		return FederatedRoundResult{}, errors.New("fedgreensub: nil coordinator")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	previous := cloneModelState(c.global)
	states := make([]ModelState, 0, len(c.peers))
	participantIDs := make([]string, 0, len(c.peers))
	resources := make([]float64, 0, len(c.peers))
	trust := make([]float64, 0, len(c.peers))
	localLoss := 0.0
	localTrainingTime := time.Duration(0)
	rejected := 0
	for _, participant := range c.peers {
		resource := participant.ResourceWeight
		if math.IsNaN(resource) || math.IsInf(resource, 0) || resource < 0 || resource > 1 {
			rejected++
			continue
		}
		// The zero value means no explicit controlled-testbed metadata was
		// supplied, so retain a neutral availability for legacy participants.
		if resource == 0 {
			resource = 1
		}
		if err := participant.Trainer.ImportWeights(c.global); err != nil {
			rejected++
			continue
		}
		dataset := participant.Dataset
		if participant.DatasetWindow != nil {
			dataset = participant.DatasetWindow.Dataset()
		}
		started := time.Now()
		result, err := participant.Trainer.Train(ctx, dataset)
		localTrainingTime += time.Since(started)
		if err != nil {
			rejected++
			continue
		}
		state, err := participant.Trainer.ExportWeights()
		if err != nil || !sameShape(state, c.global) || !modelStateFinite(state) || ValidateModelState(state, c.global.FeatureVersion, c.clipping) != nil {
			rejected++
			continue
		}
		state.Samples = int64(len(dataset.Samples))
		state.Loss = result.Loss
		states = append(states, state)
		participantIDs = append(participantIDs, participant.ID)
		resources = append(resources, resource)
		participantTrust := .5
		if participant.TrustKnown {
			participantTrust = clamp01(participant.TrustScore)
		}
		trust = append(trust, participantTrust)
		localLoss += result.Loss
	}
	if len(states) == 0 {
		return FederatedRoundResult{}, errors.New("fedgreensub: no valid participant updates")
	}
	var (
		global ModelState
		err    error
	)
	aggregationStarted := time.Now()
	switch method {
	case EnergyFedAvgMethod:
		global, err = c.agg.EnergyWeightedFedAvg(states, resources)
	case TrustFedAvgMethod:
		global, err = c.agg.TrustWeightedFedAvg(states, trust, 1)
	default:
		global, err = c.agg.FedAvg(states)
	}
	if err != nil {
		return FederatedRoundResult{}, err
	}
	aggregationTime := time.Since(aggregationStarted)
	global.FeatureVersion = c.global.FeatureVersion
	global.NormalizationVersion = c.global.NormalizationVersion
	global.ArchitectureVersion = c.global.ArchitectureVersion
	global.Version = c.round + 1
	global.Loss = localLoss / float64(len(states))
	if err := ValidateModelState(global, c.global.FeatureVersion, c.clipping); err != nil {
		return FederatedRoundResult{}, fmt.Errorf("global model rejected: %w", err)
	}
	for _, participant := range c.peers {
		_ = participant.Trainer.ImportWeights(global)
	}
	c.round++
	c.global = cloneModelState(global)
	averageLoss := localLoss / float64(len(states))
	effectiveWeights := make(map[string]float64, len(states))
	weightTotal := 0.0
	for index, state := range states {
		weight := float64(state.Samples)
		switch method {
		case EnergyFedAvgMethod:
			weight *= resources[index] * ConnectivityScore(state.PacketLossRate, state.PeerUptimeSeconds, state.SuccessfulPublishes)
		case TrustFedAvgMethod:
			trustFactor := .5
			if c.peers[participantIDs[index]].TrustKnown {
				trustFactor = clamp01(trust[index])
			}
			weight = float64(minInt64(state.Samples, 1000)) * (.1 + .9*trustFactor) * ConnectivityScore(state.PacketLossRate, state.PeerUptimeSeconds, state.SuccessfulPublishes)
		}
		effectiveWeights[participantIDs[index]] = weight
		weightTotal += weight
	}
	if weightTotal > 0 {
		for id, weight := range effectiveWeights {
			effectiveWeights[id] = weight / weightTotal
		}
	}
	return FederatedRoundResult{Round: c.round, Method: method, Global: cloneModelState(global), Contributors: len(states), Rejected: rejected, AverageLocalLoss: averageLoss, ParameterChange: modelDistance(previous, global), LocalTrainingTime: localTrainingTime, AggregationTime: aggregationTime, GlobalLoss: averageLoss, EffectiveWeights: effectiveWeights}, nil
}

func sameShape(left, right ModelState) bool {
	return len(left.Weights) == len(right.Weights) && len(left.Biases) == len(right.Biases)
}

func modelDistance(left, right ModelState) float64 {
	sum := 0.0
	for i := range left.Weights {
		if i < len(right.Weights) {
			delta := right.Weights[i] - left.Weights[i]
			sum += delta * delta
		}
	}
	for i := range left.Biases {
		if i < len(right.Biases) {
			delta := right.Biases[i] - left.Biases[i]
			sum += delta * delta
		}
	}
	return math.Sqrt(sum)
}
