package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProxyConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Config struct {
	APIKey       string      `json:"apiKey"`
	Model        string      `json:"model"`
	SystemPrompt string      `json:"systemPrompt,omitempty"`
	Proxy        *ProxyConfig `json:"proxy,omitempty"`
}

var defaults = Config{
	APIKey: "",
	Model:  "deepseek-v4-pro",
	Proxy: &ProxyConfig{
		Host: "127.0.0.1",
		Port: 8082,
	},
}

func path() string {
	if p := os.Getenv("CLAUDE_GO_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "claude-go", "config.json")
}

func Load() (Config, error) {
	cfg := defaults
	f, err := os.Open(path())
	if err != nil {
		return cfg, nil
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Proxy == nil {
		cfg.Proxy = defaults.Proxy
	}
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	return cfg, nil
}

func Save(cfg Config) error {
	if cfg.Proxy == nil {
		cfg.Proxy = defaults.Proxy
	}
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if err := os.MkdirAll(filepath.Dir(path()), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	f.Write([]byte("\n"))
	return nil
}
