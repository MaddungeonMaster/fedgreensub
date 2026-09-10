package implementations

type TrustAwareFedGreenSub struct{ adapter }

func (t *TrustAwareFedGreenSub) Name() string { return "trustaware" }
func (t *TrustAwareFedGreenSub) Start(c Config) error {
	t.adapter.start(Config{Name: t.Name(), FedGreen: true, TrustAware: true, TrustEnabled: true, FLRounds: c.FLRounds})
	return nil
}
func (t *TrustAwareFedGreenSub) RunWorkload(w Workload) error { return t.adapter.run(w) }
func (t *TrustAwareFedGreenSub) Stop() error                  { return nil }
func (t *TrustAwareFedGreenSub) Metrics() Metrics             { return t.adapter.metrics }
