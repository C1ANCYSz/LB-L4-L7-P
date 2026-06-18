package config

import "errors"

type TcpKeepAlive struct {
	Enabled    bool `json:"enabled"`
	IdleMs     int  `json:"idleMs"`
	IntervalMs int  `json:"intervalMs"`
	Count      int  `json:"count"`
}

func (t *TcpKeepAlive) Validate() error {

	if t == nil {
		return nil
	}

	if !t.Enabled || t.IdleMs == 0 || t.IntervalMs == 0 || t.Count == 0 {
		return errors.New("fields are missing")
	}
	return nil
}
