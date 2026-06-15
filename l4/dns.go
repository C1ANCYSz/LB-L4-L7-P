package l4

import (
	"lb-go/resources"
	"lb-go/utils"
	"log/slog"
	"time"
)

// StartDNSResolver starts a background goroutine that periodically resolves
// the hostnames of all backends in the current runtime pool.
func (lb *LoadBalancer) StartDNSResolver(interval time.Duration) {
	lb.Wg.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-lb.Quit:
				return
			case <-ticker.C:
				lb.ResolveAllBackends()
			}
		}
	})
}

// ResolveAllBackends resolves all backends in the current runtime pool.
func (lb *LoadBalancer) ResolveAllBackends() {
	rt := lb.Runtime.Load()
	if rt == nil || rt.BackendPool == nil {
		return
	}
	for i := range rt.BackendPool.Backends {
		lb.ResolveBackend(&rt.BackendPool.Backends[i])
	}
}

// ResolveBackend resolves the hostname of a single backend and updates its Address atomically.
func (lb *LoadBalancer) ResolveBackend(backend *resources.Backend) {
	if !backend.Resolving.CompareAndSwap(false, true) {
		// Already resolving in another goroutine
		return
	}
	defer backend.Resolving.Store(false)

	resolved, err := utils.ResolveHost(backend.OriginalAddress)
	if err != nil {
		lb.Logger.Warn("DNS resolution failed",
			slog.String("host", backend.OriginalAddress),
			slog.Any("err", err),
		)
		return
	}

	oldAddrPtr := backend.Address.Load()
	if oldAddrPtr == nil || *oldAddrPtr != resolved {
		backend.Address.Store(&resolved)
		lb.Logger.Info("DNS resolved",
			slog.String("original", backend.OriginalAddress),
			slog.String("resolved", resolved),
		)
	}
}
