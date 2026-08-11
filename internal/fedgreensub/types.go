package fedgreensub

import "time"

// RuntimeMetrics captures the local runtime and GossipSub health signals used by
// the adaptive extension layer.
type RuntimeMetrics struct {
	Timestamp time.Time

	CPU      float64
	MemoryMB float64

	Goroutines int

	BandwidthInBps  float64
	BandwidthOutBps float64

	IncomingRate  float64
	OutgoingRate  float64
	DuplicateRate float64

	MeshDegree int
	PeerCount  int

	PublishLatency      float64
	HeartbeatDuration   time.Duration
	PeerScoreAverage    float64
	BytesSent           uint64
	BytesReceived       uint64
	DroppedMessages     uint64
	SuccessfulPublishes uint64
	PacketLossRate      float64
	PeerUptimeSeconds   float64
}

// GossipParameters represents the adaptive knobs that FedGreenSub can tune at
// runtime without changing the wire protocol.
type GossipParameters struct {
	MeshDegree        int
	DLow              int
	DHigh             int
	GossipFactor      float64
	HeartbeatInterval time.Duration
	FanoutTTL         time.Duration
	PruneBackoff      time.Duration
}

// ModelState stores a serializable view of model parameters and lightweight
// training metadata for federated exchange.
type ModelState struct {
	Weights             []float64
	Biases              []float64
	Version             uint64
	Samples             int64
	Loss                float64
	EnergyEstimate      float64
	BatteryScore        float64
	PacketLossRate      float64
	PeerUptimeSeconds   float64
	SuccessfulPublishes uint64
}

// TrainingSample represents one labeled example for local training.
type TrainingSample struct {
	Features []float64
	Targets  []float64
	Weight   float64
}

// TrainingDataset is the unit passed into the local trainer.
type TrainingDataset struct {
	Samples []TrainingSample
}

// ValidationReport summarizes safety checks applied before parameters are
// pushed into the live GossipSub runtime.
type ValidationReport struct {
	Accepted bool
	Reason   string
}
