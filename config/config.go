package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"lb-go/resources"
	"log"
	"os"
)

type HealthCheck struct {
	IntervalMs       int `json:"intervalMs"`
	FailureThreshold int `json:"failureThreshold"`
	SuccessThreshold int `json:"successThreshold"`
}
type Config struct {
	ConfigBackends                []resources.ConfigBackend `json:"backends"`
	BalanceMode                   BalanceMode               `json:"balanceMode"`
	MaxConn                       int                       `json:"maxConnections"`
	MaxConcurrentConnectionsPerIP int                       `json:"maxConcurrentConnectionsPerIP"`
	RateLimit                     int                       `json:"connectionRateLimitPerMinute,omitempty"`
	IdleTimeoutMs                 *int                      `json:"idleTimeoutMs"`
	DNSRefreshIntervalMs          int                       `json:"dnsRefreshIntervalMs,omitempty"`
	TlsMode                       TLSMode                   `json:"tls"`
	ProxyProtocol                 ProxyProtocol             `json:"proxyProtocol"`
	HealthCheck                   HealthCheck               `json:"healthCheck"`
	TcpKeepAlive                  *TcpKeepAlive             `json:"tcpKeepAlive"`
	Debug                         bool                      `json:"debug"`
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
	err = config.TcpKeepAlive.Validate()
	if err != nil {
		return nil, err
	}

	if config.RateLimit == 0 {
		config.RateLimit = 0
	}
	config.logConfig()

	return config, nil

}

func (c Config) validateConfig() error {
	if !c.BalanceMode.Validate() {
		return errors.New("invalid balance mode")
	}

	if c.MaxConn == 0 {
		return errors.New("Please specify max connections")
	}

	return nil
}

func (c Config) logConfig() {
	var proxyStatus string
	if c.ProxyProtocol.Enabled {
		proxyStatus = "V" + fmt.Sprint(*c.ProxyProtocol.Version)
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
	row("Max Connections", fmt.Sprint(c.MaxConn))
	row("Debug", fmt.Sprintf("%t", c.Debug))
	if c.IdleTimeoutMs == nil {
		row("Idle Timeout (ms)", "disabled")
	} else {
		row("Idle Timeout (ms)", fmt.Sprint(*c.IdleTimeoutMs))
	}
	row("TLS", string(c.TlsMode))
	row("Proxy Protocol", proxyStatus)
	if c.TcpKeepAlive == nil {
		row("Keep Alive", "disabled")
	} else {
		idle := "default"
		if c.TcpKeepAlive.IdleMs == 0 {
			idle = fmt.Sprintf("%dms", c.TcpKeepAlive.IdleMs)
		}

		interval := "default"
		if c.TcpKeepAlive.IntervalMs == 0 {
			interval = fmt.Sprintf("%dms", c.TcpKeepAlive.IntervalMs)
		}

		count := "default"
		if c.TcpKeepAlive.Count == 0 {
			count = fmt.Sprintf("%d", c.TcpKeepAlive.Count)
		}

		row("Keep Alive", fmt.Sprintf(
			"enabled: %v idle: %s interval: %s count: %s",
			c.TcpKeepAlive.Enabled,
			idle,
			interval,
			count,
		))
	}
	row("Rate Limit", fmt.Sprint(c.RateLimit))

	log.Println("└────────────────────┴────────────────────────────┘")

	log.Println("Backends:")
	log.Println("┌─────┬────────────────────────────┐")
	log.Printf(" │ %-3s│ %-26s │\n", "#", "Address   │")
	log.Println("├─────┼────────────────────────────┤")

	for i, backend := range c.ConfigBackends {
		log.Printf("│ %-3d │ %-26s │\n", i+1, backend.Address)
	}

	log.Println("└─────┴────────────────────────────┘")
}
