package main

import "math"

type ExperimentMetrics struct {
	CPU, MemoryMB                                               float64
	BytesSent, BytesReceived                                    uint64
	Published, Received, Delivered, Duplicates                  uint64
	DeliveryRatio, DuplicateRatio                               float64
	LatencyAverage, LatencyP95                                  float64
	AverageMeshDegree                                           float64
	HeartbeatCount                                              uint64
	Energy, EnergyPerDeliveredMessage                           float64
	Available                                                   map[string]bool
	AverageTrust, MinimumTrust, TrustVariance, TrustedPeerRatio float64
	TrustUpdateTime, PeerRankingTime, TrustAggregationTime      float64
	FLRounds                                                    uint64
	TrainingLoss, GlobalLoss, TrainingTime, AggregationTime     float64
	TotalRoundTime, ParameterChange                             float64
	Contributors, Rejected                                      int
	LocalLossByRound, GlobalLossByRound, ParameterChangeByRound []float64
	TrainingTimeByRound, AggregationTimeByRound                 []float64
	EffectiveWeights                                            map[string]float64
}

func (m *ExperimentMetrics) finalize() {
	if m == nil {
		return
	}
	if m.Published > 0 {
		m.DeliveryRatio = float64(m.Delivered) / float64(m.Published)
	}
	if m.Received > 0 {
		m.DuplicateRatio = float64(m.Duplicates) / float64(m.Received)
	}
	if m.Delivered > 0 {
		m.EnergyPerDeliveredMessage = m.Energy / float64(m.Delivered)
	}
	if math.IsNaN(m.DeliveryRatio) || math.IsInf(m.DeliveryRatio, 0) {
		m.DeliveryRatio = 0
	}
	if math.IsNaN(m.DuplicateRatio) || math.IsInf(m.DuplicateRatio, 0) {
		m.DuplicateRatio = 0
	}
	if math.IsNaN(m.EnergyPerDeliveredMessage) || math.IsInf(m.EnergyPerDeliveredMessage, 0) {
		m.EnergyPerDeliveredMessage = 0
	}
}

func (m ExperimentMetrics) valid() bool {
	return m.DeliveryRatio >= 0 && m.DeliveryRatio <= 1 && m.DuplicateRatio >= 0 && m.DuplicateRatio <= 1 && m.Energy >= 0 && m.AverageTrust >= 0 && m.AverageTrust <= 1 && m.MinimumTrust >= 0 && m.MinimumTrust <= 1
}
