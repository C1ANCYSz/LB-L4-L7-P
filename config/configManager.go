package config

import (
	"log"
	"log/slog"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

type ConfigManager struct {
	Cfg atomic.Pointer[Config]

	OnReload func(*Config)
}

func NewConfigManager(cfg *Config, onReload func(*Config)) *ConfigManager {
	cm := &ConfigManager{}

	cm.Cfg.Store(cfg)
	cm.OnReload = onReload

	return cm
}
func (cm *ConfigManager) Get() *Config {
	return cm.Cfg.Load()
}

func (cm *ConfigManager) Reload() {
	newCfg, err := LoadConfig()
	if err != nil {
		log.Printf("reload failed: %v", err)
		return
	}

	cm.Cfg.Store(newCfg)

	if cm.OnReload != nil {
		cm.OnReload(newCfg)
	}
}
func (cm *ConfigManager) Watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	if err := watcher.Add("config.json"); err != nil {
		slog.Error(err.Error())
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Write) ||
				event.Has(fsnotify.Create) ||
				event.Has(fsnotify.Rename) {

				cm.Reload()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

			slog.Error(err.Error())
		}
	}
}
