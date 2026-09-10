package main

import "time"

type ExperimentConfig struct {
	Scenario      string
	Peers         int
	Messages      int
	MessageSize   int
	PublishRate   float64
	Duration      time.Duration
	Seed          int64
	Repetitions   int
	PacketLoss    float64
	Latency       time.Duration
	DuplicateRate float64
	ChurnRate     float64
	TrustEnabled  bool
	FLRounds      int
	PeerCounts    []int
}

func DefaultConfig() ExperimentConfig {
	return ExperimentConfig{Scenario: "normal", Peers: 20, Messages: 10000, MessageSize: 1024, PublishRate: 100, Duration: 100 * time.Second, Seed: 42, Repetitions: 5, Latency: 20 * time.Millisecond, FLRounds: 3}
}

func (c ExperimentConfig) normalize() ExperimentConfig {
	d := DefaultConfig()
	if c.Scenario == "" {
		c.Scenario = d.Scenario
	}
	if c.Peers <= 0 {
		c.Peers = d.Peers
	}
	if c.Messages <= 0 {
		c.Messages = d.Messages
	}
	if c.MessageSize <= 0 {
		c.MessageSize = d.MessageSize
	}
	if c.PublishRate <= 0 {
		c.PublishRate = d.PublishRate
	}
	if c.Duration <= 0 {
		c.Duration = time.Duration(float64(c.Messages) / c.PublishRate * float64(time.Second))
	}
	if c.Repetitions <= 0 {
		c.Repetitions = d.Repetitions
	}
	if c.Latency < 0 {
		c.Latency = 0
	}
	if c.PacketLoss < 0 {
		c.PacketLoss = 0
	}
	if c.PacketLoss > 1 {
		c.PacketLoss = 1
	}
	if c.DuplicateRate < 0 {
		c.DuplicateRate = 0
	}
	if c.DuplicateRate > 1 {
		c.DuplicateRate = 1
	}
	if c.ChurnRate < 0 {
		c.ChurnRate = 0
	}
	if c.ChurnRate > 1 {
		c.ChurnRate = 1
	}
	if c.FLRounds < 0 {
		c.FLRounds = 0
	}
	if len(c.PeerCounts) > 0 {
		seen := make(map[int]struct{}, len(c.PeerCounts))
		counts := make([]int, 0, len(c.PeerCounts))
		for _, peers := range c.PeerCounts {
			if peers > 0 {
				if _, ok := seen[peers]; !ok {
					seen[peers] = struct{}{}
					counts = append(counts, peers)
				}
			}
		}
		c.PeerCounts = counts
	}
	return c
}

type Workload struct {
	Peers       int
	Messages    int
	MessageSize int
	PublishRate float64
	Duration    time.Duration
	RandomSeed  int64
	PacketLoss  float64
	Latency     time.Duration
	Duplicates  float64
	Churn       float64
}

func (c ExperimentConfig) Workload() Workload {
	c = c.normalize()
	return Workload{c.Peers, c.Messages, c.MessageSize, c.PublishRate, c.Duration, c.Seed, c.PacketLoss, c.Latency, c.DuplicateRate, c.ChurnRate}
}
