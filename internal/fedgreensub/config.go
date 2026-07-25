package fedgreensub

import "time"

// Config controls the adaptive extension layer and its background workflows.
type Config struct {
	EnableFederatedLearning bool
	EnableAdaptiveMode      bool
	EnableEnergyEstimator   bool

	TrainingInterval    time.Duration
	AggregationInterval time.Duration
	PredictionInterval  time.Duration
	MetricsInterval     time.Duration

	EMAAlpha float64

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
		EnableFederatedLearning: false,
		EnableAdaptiveMode:      false,
		EnableEnergyEstimator:   false,
		TrainingInterval:        10 * time.Minute,
		AggregationInterval:     15 * time.Minute,
		PredictionInterval:      1 * time.Second,
		MetricsInterval:         1 * time.Second,
		EMAAlpha:                0.25,
		MinMeshDegree:           3,
		MaxMeshDegree:           16,
		MinHeartbeatInterval:    500 * time.Millisecond,
		MaxHeartbeatInterval:    10 * time.Second,
		MinGossipFactor:         0.1,
		MaxGossipFactor:         0.5,
		EnergyWeights:           DefaultEnergyWeights(),
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
	if c.MinMeshDegree < 3 {
		c.MinMeshDegree = 3
	}
	if c.MaxMeshDegree < c.MinMeshDegree {
		c.MaxMeshDegree = c.MinMeshDegree
	}
	if c.MinHeartbeatInterval <= 0 {
		c.MinHeartbeatInterval = DefaultConfig().MinHeartbeatInterval
	}
	if c.MaxHeartbeatInterval < c.MinHeartbeatInterval {
		c.MaxHeartbeatInterval = c.MinHeartbeatInterval
	}
	if c.MinGossipFactor <= 0 {
		c.MinGossipFactor = DefaultConfig().MinGossipFactor
	}
	if c.MaxGossipFactor < c.MinGossipFactor {
		c.MaxGossipFactor = c.MinGossipFactor
	}
	if c.EnergyWeights == (EnergyWeights{}) {
		c.EnergyWeights = DefaultEnergyWeights()
	}
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

func WithEMAAlpha(alpha float64) Option {
	return func(cfg *Config) {
		cfg.EMAAlpha = alpha
	}
}

func WithMeshBounds(minMeshDegree, maxMeshDegree int) Option {
	return func(cfg *Config) {
		cfg.MinMeshDegree = minMeshDegree
		cfg.MaxMeshDegree = maxMeshDegree
	}
}

func WithHeartbeatBounds(minInterval, maxInterval time.Duration) Option {
	return func(cfg *Config) {
		cfg.MinHeartbeatInterval = minInterval
		cfg.MaxHeartbeatInterval = maxInterval
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
