package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const configFileName = ".gatorconfig.json"

// Config represents the JSON file structure
type Config struct {
	DbURL       string `json:"db_url"`
	CurrentUser string `json:"current_user_name"`
}

// Read reads the JSON file found at ~/.gatorconfig.json and returns a Config struct
func Read() (Config, error) {
	var cfg Config
	
	configPath, err := getConfigFilePath()
	if err != nil {
		return cfg, err
	}
	
	file, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, err
	}
	
	err = json.Unmarshal(file, &cfg)
	if err != nil {
		return cfg, err
	}
	
	return cfg, nil
}

// SetUser writes the config struct to the JSON file after setting the current_user_name field
func (c *Config) SetUser(username string) error {
	c.CurrentUser = username
	return write(*c)
}

// getConfigFilePath returns the full path to the config file
func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, configFileName), nil
}

// write writes the config struct to the JSON file
func write(cfg Config) error {
	configPath, err := getConfigFilePath()
	if err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}