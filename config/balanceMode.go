package config

type BalanceMode string

const (
	RoundRobin BalanceMode = "roundRobin"
	LeastConn  BalanceMode = "leastConnections"
	IpHash     BalanceMode = "ipHash"
)

func (m BalanceMode) Validate() bool {
	switch m {
	case RoundRobin, LeastConn, IpHash:
		return true

	}
	return false
}
