package implementations

type FedGreenSub struct{ adapter }

func (f *FedGreenSub) Name() string { return "fedgreen" }
func (f *FedGreenSub) Start(c Config) error {
	f.adapter.start(Config{Name: f.Name(), FedGreen: true, FLRounds: c.FLRounds})
	return nil
}
func (f *FedGreenSub) RunWorkload(w Workload) error { return f.adapter.run(w) }
func (f *FedGreenSub) Stop() error                  { return nil }
func (f *FedGreenSub) Metrics() Metrics             { return f.adapter.metrics }
