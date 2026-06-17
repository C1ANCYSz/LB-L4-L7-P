package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
)

type Backend struct {
	address      string
	pingEndpoint string
}
type Config struct {
	BackendsUrls         []string      `json:"backends"`
	BalanceMode          BalanceMode   `json:"balanceMode"`
	LBLevel              LBLevel       `json:"level"`
	MaxConn              int           `json:"maxConnections"`
	IdleTimeoutMs        int           `json:"idleTimeoutMs"`
	PingIntervalMs       int           `json:"pingIntervalMs"`
	DNSRefreshIntervalMs int           `json:"dnsRefreshIntervalMs,omitempty"`
	Debug                bool          `json:"debug"`
	TlsMode              TLSMode       `json:"tls"`
	ProxyProtocol        ProxyProtocol `json:"proxyProtocol"`
}

func LoadConfig() (*Config, error) {
	var config *Config
	file, err := os.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}
	err = config.validateConfig()
	if err != nil {
		return nil, err
	}
	err = config.TlsMode.Validate()
	if err != nil {
		return nil, err
	}

	err = config.ProxyProtocol.Validate()
	if err != nil {
		return nil, err
	}
	if config.DNSRefreshIntervalMs == 0 {
		config.DNSRefreshIntervalMs = 15000 // 15 seconds default
	}
	config.logConfig()

	return config, nil

}

func (c Config) validateConfig() error {
	if !c.BalanceMode.Validate() {
		return errors.New("invalid balance mode")
	}
	if !c.LBLevel.Validate() {
		return errors.New("Invalid LB Level")
	}
	if c.MaxConn == 0 {
		return errors.New("Please specify max connections")
	}
	if c.PingIntervalMs == 0 {
		return errors.New("Please specify ping interval")
	}
	return nil
}

func (c Config) logConfig() {
	var proxyStatus string
	if c.ProxyProtocol.Enabled {
		proxyStatus = "V" + fmt.Sprint(c.ProxyProtocol.Version)
	} else {
		proxyStatus = "Disabled"
	}

	row := func(label, value string) {
		log.Printf("│ %-18s │ %-26s │\n", label, value)
	}
	log.Println("┌────────────────────┬────────────────────────────┐")
	log.Printf("│ %-18s │ %-26s │\n", "Setting", "Value")
	log.Println("├────────────────────┼────────────────────────────┤")

	row("Balance Mode", string(c.BalanceMode))
	row("LB Level", fmt.Sprint(c.LBLevel))
	row("Max Connections", fmt.Sprint(c.MaxConn))
	row("Debug", fmt.Sprintf("%t", c.Debug))
	row("Ping Interval (ms)", fmt.Sprint(c.PingIntervalMs))
	row("Idle Timeout (ms)", fmt.Sprint(c.IdleTimeoutMs))
	row("TLS", string(c.TlsMode))
	row("Proxy Protocol", proxyStatus)

	log.Println("└────────────────────┴────────────────────────────┘")

	log.Println("Backends:")
	log.Println("┌─────┬────────────────────────────┐")
	log.Printf("│ %-3s │ %-26s │\n", "#", "Address")
	log.Println("├─────┼────────────────────────────┤")

	for i, backend := range c.BackendsUrls {
		log.Printf("│ %-3d │ %-26s │\n", i+1, backend)
	}

	log.Println("└─────┴────────────────────────────┘")
}
