package fedgreensub

import "github.com/libp2p/go-libp2p/core/peer"

type TrustFeatures struct {
	AverageNeighborTrust float64
	MinimumNeighborTrust float64
	TrustVariance        float64
	TrustedPeerRatio     float64
}

func (f TrustFeatures) FeatureVector() []float64 {
	return []float64{f.AverageNeighborTrust, f.MinimumNeighborTrust, f.TrustVariance, f.TrustedPeerRatio}
}

func AggregateTrustFeatures(scores map[peer.ID]TrustScore, trustedThreshold float64) TrustFeatures {
	if len(scores) == 0 {
		return TrustFeatures{AverageNeighborTrust: .5, MinimumNeighborTrust: .5}
	}
	if trustedThreshold < 0 || trustedThreshold > 1 {
		trustedThreshold = .7
	}
	mean, minimum, trusted := 0.0, 1.0, 0
	for _, score := range scores {
		value := clamp01(score.Score)
		mean += value
		if value < minimum {
			minimum = value
		}
		if value >= trustedThreshold {
			trusted++
		}
	}
	mean /= float64(len(scores))
	variance := 0.0
	for _, score := range scores {
		delta := clamp01(score.Score) - mean
		variance += delta * delta
	}
	variance /= float64(len(scores))
	return TrustFeatures{mean, minimum, variance, float64(trusted) / float64(len(scores))}
}

func TrustMetrics(base RuntimeMetrics, features TrustFeatures) RuntimeMetrics {
	base.AverageNeighborTrust, base.MinimumNeighborTrust = clamp01(features.AverageNeighborTrust), clamp01(features.MinimumNeighborTrust)
	base.TrustVariance, base.TrustedPeerRatio = clamp01(features.TrustVariance), clamp01(features.TrustedPeerRatio)
	return base
}
