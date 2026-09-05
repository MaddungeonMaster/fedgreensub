package fedgreensub

import (
	"log/slog"
	"time"
)

// Config controls the adaptive extension layer and its background workflows.
type Config struct {
	EnableFederatedLearning  bool
	EnableAdaptiveMode       bool
	EnableEnergyEstimator    bool
	EnableTrustScore         bool
	TrustWeights             TrustWeights
	TrustEMAAlpha            float64
	MinimumTrust             float64
	PeerScoreWeight          float64
	TrustScoreWeight         float64
	TrustAggregationWeight   float64
	TrustedPeerThreshold     float64
	ModelUpdateClipping      float64
	TrustObservationInterval time.Duration

	TrainingInterval    time.Duration
	AggregationInterval time.Duration
	PredictionInterval  time.Duration
	MetricsInterval     time.Duration

	EMAAlpha float64
	Logger   *slog.Logger

	MinMeshDegree int
	MaxMeshDegree int

	MinHeartbeatInterval time.Duration
	MaxHeartbeatInterval time.Duration
	MinGossipFactor      float64
	MaxGossipFactor      float64

	EnergyWeights EnergyWeights
}

// Option configures a Config instance using the functional options pattern.
type Option func(*Config)

// DefaultConfig returns a conservative configuration that keeps the adaptive
// layer disabled until explicitly enabled by the caller.
func DefaultConfig() Config {
	return Config{
		EnableFederatedLearning:  false,
		EnableAdaptiveMode:       false,
		EnableEnergyEstimator:    false,
		EnableTrustScore:         false,
		TrustWeights:             DefaultTrustWeights(),
		TrustEMAAlpha:            0.25,
		MinimumTrust:             0.1,
		PeerScoreWeight:          0.5,
		TrustScoreWeight:         0.5,
		TrustAggregationWeight:   1,
		TrustedPeerThreshold:     0.7,
		ModelUpdateClipping:      10,
		TrustObservationInterval: time.Second,
		TrainingInterval:         10 * time.Minute,
		AggregationInterval:      15 * time.Minute,
		PredictionInterval:       1 * time.Second,
		MetricsInterval:          1 * time.Second,
		EMAAlpha:                 0.25,
		Logger:                   slog.Default(),
		MinMeshDegree:            3,
		MaxMeshDegree:            16,
		MinHeartbeatInterval:     500 * time.Millisecond,
		MaxHeartbeatInterval:     10 * time.Second,
		MinGossipFactor:          0.1,
		MaxGossipFactor:          0.5,
		EnergyWeights:            DefaultEnergyWeights(),
	}
}

// NewConfig builds a validated Config from the provided options.
func NewConfig(opts ...Option) Config {
	cfg := DefaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	cfg.normalize()
	return cfg
}

func (c *Config) normalize() {
	if c == nil {
		return
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.TrainingInterval <= 0 {
		c.TrainingInterval = DefaultConfig().TrainingInterval
	}
	if c.AggregationInterval <= 0 {
		c.AggregationInterval = DefaultConfig().AggregationInterval
	}
	if c.PredictionInterval <= 0 {
		c.PredictionInterval = DefaultConfig().PredictionInterval
	}
	if c.MetricsInterval <= 0 {
		c.MetricsInterval = DefaultConfig().MetricsInterval
	}
	if c.EMAAlpha <= 0 || c.EMAAlpha > 1 {
		c.EMAAlpha = DefaultConfig().EMAAlpha
	}
	if c.TrustEMAAlpha <= 0 || c.TrustEMAAlpha > 1 {
		c.TrustEMAAlpha = DefaultConfig().TrustEMAAlpha
	}
	c.TrustWeights = c.TrustWeights.normalize()
	if c.MinimumTrust < 0 || c.MinimumTrust > 1 {
		c.MinimumTrust = DefaultConfig().MinimumTrust
	}
	if c.PeerScoreWeight < 0 || c.TrustScoreWeight < 0 || c.PeerScoreWeight+c.TrustScoreWeight <= 0 {
		c.PeerScoreWeight, c.TrustScoreWeight = .5, .5
	}
	if c.TrustAggregationWeight < 0 {
		c.TrustAggregationWeight = 0
	}
	if c.TrustedPeerThreshold < 0 || c.TrustedPeerThreshold > 1 {
		c.TrustedPeerThreshold = DefaultConfig().TrustedPeerThreshold
	}
	if c.ModelUpdateClipping <= 0 {
		c.ModelUpdateClipping = DefaultConfig().ModelUpdateClipping
	}
	if c.TrustObservationInterval <= 0 {
		c.TrustObservationInterval = DefaultConfig().TrustObservationInterval
	}
	if c.MinMeshDegree <= 0 || c.MinMeshDegree < 3 {
		c.MinMeshDegree = 3
	}
	if c.MaxMeshDegree <= 0 || c.MaxMeshDegree < c.MinMeshDegree {
		c.MaxMeshDegree = c.MinMeshDegree
	}
	if c.MinHeartbeatInterval <= 0 {
		c.MinHeartbeatInterval = DefaultConfig().MinHeartbeatInterval
	}
	if c.MaxHeartbeatInterval <= 0 || c.MaxHeartbeatInterval < c.MinHeartbeatInterval {
		c.MaxHeartbeatInterval = c.MinHeartbeatInterval
	}
	if c.MinGossipFactor <= 0 {
		c.MinGossipFactor = DefaultConfig().MinGossipFactor
	}
	if c.MaxGossipFactor <= 0 || c.MaxGossipFactor < c.MinGossipFactor {
		c.MaxGossipFactor = c.MinGossipFactor
	}
	if c.EnergyWeights == (EnergyWeights{}) {
		c.EnergyWeights = DefaultEnergyWeights()
	}
}

func WithTrustScore(enabled bool) Option { return func(cfg *Config) { cfg.EnableTrustScore = enabled } }
func WithTrustWeights(weights TrustWeights) Option {
	return func(cfg *Config) { cfg.TrustWeights = weights }
}
func WithTrustEMAAlpha(alpha float64) Option { return func(cfg *Config) { cfg.TrustEMAAlpha = alpha } }
func WithMinimumTrust(value float64) Option  { return func(cfg *Config) { cfg.MinimumTrust = value } }
func WithPeerScoreWeight(value float64) Option {
	return func(cfg *Config) { cfg.PeerScoreWeight = value }
}
func WithTrustScoreWeight(value float64) Option {
	return func(cfg *Config) { cfg.TrustScoreWeight = value }
}
func WithTrustAggregationWeight(value float64) Option {
	return func(cfg *Config) { cfg.TrustAggregationWeight = value }
}
func WithTrustedPeerThreshold(value float64) Option {
	return func(cfg *Config) { cfg.TrustedPeerThreshold = value }
}
func WithModelUpdateClipping(value float64) Option {
	return func(cfg *Config) { cfg.ModelUpdateClipping = value }
}
func WithTrustObservationInterval(value time.Duration) Option {
	return func(cfg *Config) { cfg.TrustObservationInterval = value }
}

func WithFederatedLearning(enabled bool) Option {
	return func(cfg *Config) {
		cfg.EnableFederatedLearning = enabled
	}
}

func WithAdaptiveMode(enabled bool) Option {
	return func(cfg *Config) {
		cfg.EnableAdaptiveMode = enabled
	}
}

func WithEnergyEstimator(enabled bool) Option {
	return func(cfg *Config) {
		cfg.EnableEnergyEstimator = enabled
	}
}

func WithTrainingInterval(interval time.Duration) Option {
	return func(cfg *Config) {
		cfg.TrainingInterval = interval
	}
}

func WithAggregationInterval(interval time.Duration) Option {
	return func(cfg *Config) {
		cfg.AggregationInterval = interval
	}
}

func WithPredictionInterval(interval time.Duration) Option {
	return func(cfg *Config) {
		cfg.PredictionInterval = interval
	}
}

func WithMetricsInterval(interval time.Duration) Option {
	return func(cfg *Config) {
		cfg.MetricsInterval = interval
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(cfg *Config) {
		if logger != nil {
			cfg.Logger = logger
		}
	}
}

func WithEMAAlpha(alpha float64) Option {
	return func(cfg *Config) {
		cfg.EMAAlpha = alpha
	}
}

func WithMinMeshDegree(minMeshDegree int) Option {
	return func(cfg *Config) {
		cfg.MinMeshDegree = minMeshDegree
		if cfg.MaxMeshDegree < cfg.MinMeshDegree {
			cfg.MaxMeshDegree = cfg.MinMeshDegree
		}
	}
}

func WithMaxMeshDegree(maxMeshDegree int) Option {
	return func(cfg *Config) {
		cfg.MaxMeshDegree = maxMeshDegree
		if cfg.MinMeshDegree > cfg.MaxMeshDegree {
			cfg.MinMeshDegree = cfg.MaxMeshDegree
		}
	}
}

func WithMeshBounds(minMeshDegree, maxMeshDegree int) Option {
	return func(cfg *Config) {
		cfg.MinMeshDegree = minMeshDegree
		cfg.MaxMeshDegree = maxMeshDegree
	}
}

func WithMinHeartbeatInterval(minInterval time.Duration) Option {
	return func(cfg *Config) {
		cfg.MinHeartbeatInterval = minInterval
		if cfg.MaxHeartbeatInterval < cfg.MinHeartbeatInterval {
			cfg.MaxHeartbeatInterval = cfg.MinHeartbeatInterval
		}
	}
}

func WithMaxHeartbeatInterval(maxInterval time.Duration) Option {
	return func(cfg *Config) {
		cfg.MaxHeartbeatInterval = maxInterval
		if cfg.MinHeartbeatInterval > cfg.MaxHeartbeatInterval {
			cfg.MinHeartbeatInterval = cfg.MaxHeartbeatInterval
		}
	}
}

func WithHeartbeatBounds(minInterval, maxInterval time.Duration) Option {
	return func(cfg *Config) {
		cfg.MinHeartbeatInterval = minInterval
		cfg.MaxHeartbeatInterval = maxInterval
	}
}

func WithMinGossipFactor(minFactor float64) Option {
	return func(cfg *Config) {
		cfg.MinGossipFactor = minFactor
		if cfg.MaxGossipFactor < cfg.MinGossipFactor {
			cfg.MaxGossipFactor = cfg.MinGossipFactor
		}
	}
}

func WithMaxGossipFactor(maxFactor float64) Option {
	return func(cfg *Config) {
		cfg.MaxGossipFactor = maxFactor
		if cfg.MinGossipFactor > cfg.MaxGossipFactor {
			cfg.MinGossipFactor = cfg.MaxGossipFactor
		}
	}
}

func WithGossipFactorBounds(minFactor, maxFactor float64) Option {
	return func(cfg *Config) {
		cfg.MinGossipFactor = minFactor
		cfg.MaxGossipFactor = maxFactor
	}
}

func WithEnergyWeights(weights EnergyWeights) Option {
	return func(cfg *Config) {
		cfg.EnergyWeights = weights
	}
}
