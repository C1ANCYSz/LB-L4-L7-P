package l4

import (
	"lb-go/config"
	"lb-go/resources"
)

func handleBalanceMode(rt *config.Runtime, clientIP string) *resources.Backend {
	var backend *resources.Backend
	switch rt.Config.BalanceMode {
	case config.IpHash:
		backend = rt.BackendPool.GetIPHashServer(clientIP)
	case config.RoundRobin:
		backend = rt.BackendPool.GetRoundRobinServer()
	default:
		backend = rt.BackendPool.GetLeastConnServer()
	}

	return backend

}
