package config

import "errors"

type TLSMode string

const (
	TLSPassthrough TLSMode = "passthrough"
	TLSInspect     TLSMode = "inspect"
	TLSTerminate   TLSMode = "terminate"
	ReEncrypt      TLSMode = "re-encrypt"
)

func (m TLSMode) Validate() error {
	switch m {
	case TLSPassthrough, TLSInspect, TLSTerminate, ReEncrypt:
		return nil
	}
	return errors.New("Invalid TLS mode")
}
