package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	Language   string `json:"language"`
	WindowSize struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"window_size"`
	RecentFiles []string `json:"recent_files"`
}

func LoadConfig() (*AppConfig, error) {
	configPath, _ := os.UserConfigDir()
	configPath = filepath.Join(configPath, "manufacturers-db", "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return &AppConfig{}, err
	}

	var cfg AppConfig
	return &cfg, json.Unmarshal(data, &cfg)
}

func SaveConfig(cfg *AppConfig) error {
	configPath, _ := os.UserConfigDir()
	configPath = filepath.Join(configPath, "manufacturers-db")
	os.MkdirAll(configPath, 0755)
	configPath = filepath.Join(configPath, "config.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
