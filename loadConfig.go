package main

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	Servers     []string    `json:"servers"`
	BalanceMode BalanceMode `json:"balanceMode"`
	LBLevel     LBLevel     `json:"level"`
	MaxConn     int         `json:"maxConnections"`
}

func LoadConfig() ([]string, BalanceMode, LBLevel, int) {
	var config Config
	file, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatal("couldn't find config file")
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("========== REGISTERED ON ==========")
	for _, serverUrl := range config.Servers {
		log.Println(serverUrl)
	}
	if config.MaxConn == 0 {
		config.MaxConn = 5000
	}
	log.Println("BALANCE MODE: ", config.BalanceMode)
	log.Println("LB LEVEL: ", config.LBLevel)
	log.Println("MAX CONNECTIONS: ", config.MaxConn)

	return config.Servers, config.BalanceMode, config.LBLevel, config.MaxConn

}
