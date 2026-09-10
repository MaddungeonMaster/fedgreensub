package implementations

import fedgreensub "golang_project/internal/fedgreensub"

type FederatedGossipSub struct{ adapter }

func (f *FederatedGossipSub) Name() string { return "fl" }
func (f *FederatedGossipSub) Start(c Config) error {
	f.adapter.start(Config{Name: f.Name(), FedGreen: true, FLRounds: c.FLRounds, Aggregation: fedgreensub.FedAvgMethod})
	return nil
}
func (f *FederatedGossipSub) RunWorkload(w Workload) error { return f.adapter.run(w) }
func (f *FederatedGossipSub) Stop() error                  { return nil }
func (f *FederatedGossipSub) Metrics() Metrics             { return f.adapter.metrics }
