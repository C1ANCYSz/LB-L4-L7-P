package main

// func (lb *LoadBalancer) startReResolver(originalURLs []string, interval time.Duration) {
// 	go func() {
// 		ticker := time.NewTicker(interval)
// 		defer ticker.Stop()
// 		for range ticker.C {
// 			lb.reResolveServers(originalURLs)
// 		}
// 	}()
// }
// func (lb *LoadBalancer) reResolveServers(originalURLs []string) {
// 	rt := lb.runtime.Load()

// 	for i, original := range originalURLs {
// 		resolved, err := utils.ResolveHost(original)
// 		if err != nil {
// 			continue
// 		}
// 		r := resolved // new variable each iteration
// 		rt.BackendPool.backends[i].address = r

// 	}
// }
