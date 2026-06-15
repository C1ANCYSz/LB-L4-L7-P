package config

import (
	"log"
	"log/slog"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

type ConfigManager struct {
	Cfg      atomic.Pointer[Config]
	Logger   *slog.Logger
	OnReload func(*Config)
}

func NewConfigManager(cfg *Config, logger *slog.Logger) *ConfigManager {
	cm := &ConfigManager{
		Logger: logger,
	}

	cm.Cfg.Store(cfg)

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
		cm.Logger.Error(err.Error())
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

			cm.Logger.Error(err.Error())
		}
	}
}
