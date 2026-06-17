package config

import "errors"

type ProxyProtocolVersion int

const (
	V1 ProxyProtocolVersion = 1
	V2 ProxyProtocolVersion = 2
)

type ProxyProtocol struct {
	Enabled bool                  `json:"enabled"`
	Version *ProxyProtocolVersion `json:"version"`
}

func (v ProxyProtocol) Validate() error {
	if !v.Enabled {
		return nil // version irrelevant
	}
	if v.Version == nil {
		return errors.New("proxy protocol enabled but no version specified")
	}
	switch *v.Version {
	case V1, V2:
		return nil
	default:
		return errors.New("invalid proxy protocol version: must be 1 or 2")
	}
}
