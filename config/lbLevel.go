package config

type LBLevel string

const (
	TCP  LBLevel = "TCP/UDP"
	HTTP LBLevel = "HTTP"
)

func (l LBLevel) Validate() bool {
	switch l {
	case TCP, HTTP:
		return true

	}
	return false
}
