package fedgreensub

import (
	"context"
	"errors"
	"time"
)

// CandidateEvaluator supplies observed or controlled outcomes for safe
// parameter candidates. It is intentionally independent of the heuristic
// predictor so targets come from measured outcomes.
type CandidateEvaluator interface {
	Evaluate(context.Context, RuntimeMetrics, GossipParameters) (CandidateScore, error)
}

// GenerateTrainingSample evaluates safe candidates and chooses the lowest
// measured objective as the supervised target. It never calls a predictor.
func GenerateTrainingSample(ctx context.Context, metrics RuntimeMetrics, current GossipParameters, evaluator CandidateEvaluator, cfg Config) (TrainingSample, CandidateScore, error) {
	if evaluator == nil {
		return TrainingSample{}, CandidateScore{}, errors.New("fedgreensub: nil candidate evaluator")
	}
	cfg.normalize()
	candidates := CandidateParameters(current, cfg)
	scores := make([]CandidateScore, 0, len(candidates))
	for _, candidate := range candidates {
		score, err := evaluator.Evaluate(ctx, metrics, candidate)
		if err != nil {
			continue
		}
		if score.Parameters == (GossipParameters{}) {
			score.Parameters = candidate
		}
		scores = append(scores, score)
	}
	best, ok := SelectBestCandidate(scores, cfg.MinimumDeliveryRatio, cfg.TargetWeights, current)
	if !ok {
		return TrainingSample{}, CandidateScore{}, errors.New("fedgreensub: no valid candidate score")
	}
	return TrainingSample{Features: FeatureVector(metrics, cfg), Targets: ParametersTarget(best.Parameters, cfg, current), Weight: 1}, best, nil
}

func CandidateParameters(current GossipParameters, cfg Config) []GossipParameters {
	cfg.normalize()
	meshValues := []int{current.MeshDegree - 2, current.MeshDegree, current.MeshDegree + 2}
	factorValues := []float64{current.GossipFactor - .05, current.GossipFactor, current.GossipFactor + .05}
	intervalValues := []int64{int64(current.HeartbeatInterval * 3 / 4), int64(current.HeartbeatInterval), int64(current.HeartbeatInterval * 5 / 4)}
	result := make([]GossipParameters, 0, len(meshValues)*len(factorValues))
	for _, mesh := range meshValues {
		mesh = clampInt(mesh, cfg.MinMeshDegree, cfg.MaxMeshDegree)
		for _, factor := range factorValues {
			for _, interval := range intervalValues {
				candidate := GossipParameters{MeshDegree: mesh, DLow: clampInt(mesh/2, cfg.MinMeshDegree, mesh), DHigh: clampInt(mesh+4, mesh, cfg.MaxMeshDegree), GossipFactor: clampFloat(factor, cfg.MinGossipFactor, cfg.MaxGossipFactor), HeartbeatInterval: clampDuration(time.Duration(interval), cfg.MinHeartbeatInterval, cfg.MaxHeartbeatInterval)}
				result = append(result, candidate)
			}
		}
	}
	return result
}
