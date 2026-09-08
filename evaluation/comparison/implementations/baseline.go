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
