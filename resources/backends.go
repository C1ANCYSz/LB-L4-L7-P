package resources

import (
	"hash/fnv"
	"sync/atomic"
)

type Backend struct {
	Up              atomic.Bool
	OriginalAddress string
	Address         atomic.Pointer[string]
	PingEndpoint    string
	Connections     atomic.Int64
	Resolving       atomic.Bool
}
type BackendPool struct {
	Backends []Backend
}

func (bPool *BackendPool) GetIPHashServer(ip string) *Backend {
	h := fnv.New32a()
	h.Write([]byte(ip))
	idx := int(h.Sum32()) % len(bPool.Backends)

	if bPool.Backends[idx].Up.Load() {
		return &bPool.Backends[idx]
	}
	return bPool.GetLeastConnServer()
}
func (bPool *BackendPool) GetLeastConnServer() *Backend {

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

func NewBackendPool(backendsUrls []string) *BackendPool {
	backends := make([]Backend, len(backendsUrls))

	for i, url := range backendsUrls {
		backends[i].OriginalAddress = url
		u := url
		backends[i].Address.Store(&u)
		backends[i].Up.Store(false)
	}

	return &BackendPool{
		Backends: backends,
	}
}
