package resources

import (
	"hash/fnv"
	"sync/atomic"
)

type ConfigBackend struct {
	Address string
	MaxConn int
}

type Backend struct {
	Up                  atomic.Bool
	OriginalAddress     string
	Address             atomic.Pointer[string]
	Connections         atomic.Int64
	Resolving           atomic.Bool
	ConsecutiveFailures atomic.Int32
	ConsecutiveSuccess  atomic.Int32
}
type BackendPool struct {
	Backends     []Backend
	RobinCounter atomic.Uint64
}

func (p *BackendPool) GetRoundRobinServer() *Backend {
	n := uint64(len(p.Backends))
	if n == 0 {
		return nil
	}
	idx := p.RobinCounter.Add(1)
	for i := range n {
		backend := &p.Backends[(idx+i)%n]
		if backend.Up.Load() {
			return backend
		}
	}
	return p.GetLeastConnServer()
}
func (bPool *BackendPool) GetIPHashServer(ip string) *Backend {
	if len(bPool.Backends) == 0 {
		return nil
	}
	h := fnv.New32a()
	h.Write([]byte(ip))
	idx := int(h.Sum32()) % len(bPool.Backends)

	if bPool.Backends[idx].Up.Load() {
		return &bPool.Backends[idx]
	}
	return bPool.GetLeastConnServer()
}
func (bPool *BackendPool) GetLeastConnServer() *Backend {
	if len(bPool.Backends) == 0 {
		return nil
	}
	var backend *Backend
	for i := range bPool.Backends {
		s := &bPool.Backends[i]
		if !s.Up.Load() {
			continue
		}
		if backend == nil || s.Connections.Load() < backend.Connections.Load() {
			backend = s
		}
	}

	return backend
}

func NewBackendPool(backendsUrls []ConfigBackend) *BackendPool {
	backends := make([]Backend, len(backendsUrls))

	for i, backend := range backendsUrls {
		backends[i].OriginalAddress = backend.Address
		u := backend.Address
		backends[i].Address.Store(&u)
		backends[i].Up.Store(false)
	}

	return &BackendPool{
		Backends: backends,
	}
}
