package main

import (
	"encoding/json"
	"fmt"
)

type LBLevel string

const (
	TCP  LBLevel = "TCP/UDP"
	HTTP LBLevel = "HTTP"
)

func (m *LBLevel) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch LBLevel(s) {
	case TCP, HTTP:
		*m = LBLevel(s)
		return nil
	default:
		return fmt.Errorf("invalid balance mode: %q\n valid modes are: %v", s, []LBLevel{TCP, HTTP})
	}
}
