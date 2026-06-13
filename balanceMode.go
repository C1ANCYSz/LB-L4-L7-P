package main

import (
	"encoding/json"
	"fmt"
)

type BalanceMode string

const (
	RoundRobin BalanceMode = "roundRobin"
	LeastConn  BalanceMode = "leastConnections"
	IpHash     BalanceMode = "ipHash"
)

func (m *BalanceMode) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch BalanceMode(s) {
	case RoundRobin, LeastConn, IpHash:
		//deref the pointer so that unmarshal can unmarshal in the address i'm unmarshling into in the config struct
		*m = BalanceMode(s)
		return nil
	default:
		return fmt.Errorf("invalid balance mode: %q\n valid modes are: %v", s, []BalanceMode{RoundRobin, LeastConn, IpHash})
	}
}
