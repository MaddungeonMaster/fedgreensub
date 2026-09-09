package implementations

type BaselineGossipSub struct{ adapter }

func (b *BaselineGossipSub) Name() string { return "gossipsub" }
func (b *BaselineGossipSub) Start(c Config) error {
	b.adapter.start(Config{Name: b.Name()})
	return nil
}
func (b *BaselineGossipSub) RunWorkload(w Workload) error { return b.adapter.run(w) }
func (b *BaselineGossipSub) Stop() error                  { return nil }
func (b *BaselineGossipSub) Metrics() Metrics             { return b.adapter.metrics }

type HeuristicGossipSub struct{ adapter }

func (h *HeuristicGossipSub) Name() string { return "heuristic" }
func (h *HeuristicGossipSub) Start(c Config) error {
	h.adapter.start(Config{Name: h.Name(), FedGreen: true, Heuristic: true})
	return nil
}
func (h *HeuristicGossipSub) RunWorkload(w Workload) error { return h.adapter.run(w) }
func (h *HeuristicGossipSub) Stop() error                  { return nil }
func (h *HeuristicGossipSub) Metrics() Metrics             { return h.adapter.metrics }
