package config

import (
	"encoding/json"
	"errors"
	"lb-go/utils"
	"log"
	"os"
)

type Backend struct {
	address      string
	pingEndpoint string
}
type Config struct {
	BackendsUrls   []string    `json:"backends"`
	BalanceMode    BalanceMode `json:"balanceMode"`
	LBLevel        LBLevel     `json:"level"`
	MaxConn        int         `json:"maxConnections"`
	IdleTimeoutMs  int         `json:"idleTimeoutMs"`
	PingIntervalMs int         `json:"pingIntervalMs"`
	Debug          bool        `json:"debug"`
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
	resolvedUrls := utils.ResolveUrls(config.BackendsUrls)
	config.BackendsUrls = resolvedUrls
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
	log.Println("┌────────────────────┬────────────────────────────┐")
	log.Printf("│ %-18s │ %-26s │\n", "Setting", "Value")
	log.Println("├────────────────────┼────────────────────────────┤")

	log.Printf("│ %-18s │ %-26s │\n", "Balance Mode", c.BalanceMode)
	log.Printf("│ %-18s │ %-26v │\n", "LB Level", c.LBLevel)
	log.Printf("│ %-18s │ %-26d │\n", "Max Connections", c.MaxConn)
	log.Printf("│ %-18s │ %-26t │\n", "Debug", c.Debug)
	log.Printf("│ %-18s │ %-26d │\n", "Ping Interval", c.PingIntervalMs)
	log.Printf("│ %-18s │ %-26d │\n", "Idle Timeout ", c.PingIntervalMs)

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
