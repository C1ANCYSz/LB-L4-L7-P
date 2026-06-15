package config

import "lb-go/resources"

type Runtime struct {
	Config      *Config
	BackendPool *resources.BackendPool
}

func NewRuntime(cfg *Config) *Runtime {
	return &Runtime{
		Config:      cfg,
		BackendPool: resources.NewBackendPool(cfg.BackendsUrls),
	}
}
