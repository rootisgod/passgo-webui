package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type LLMConfig struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key,omitempty"`
	Model    string `json:"model"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type Config struct {
	Listen        string            `json:"listen"`
	CloudInitDir  string            `json:"cloud_init_dir"`
	CloudInitRepo string            `json:"cloud_init_repo"`
	Username      string            `json:"username"`
	Password      string            `json:"password"`
	Groups        []string          `json:"groups,omitempty"`
	VMGroups      map[string]string `json:"vm_groups,omitempty"`
	LLM           *LLMConfig        `json:"llm,omitempty"`
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".passgo-web/config.json"
	}
	return filepath.Join(home, ".passgo-web", "config.json")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.CloudInitDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.CloudInitDir = filepath.Join(home, ".passgo-web", "cloud-init")
		}
	}
	if cfg.Username == "" {
		cfg.Username = "admin"
	}
	if cfg.Password == "" {
		cfg.Password = "admin"
	}
	if cfg.Groups == nil {
		cfg.Groups = []string{}
	}
	if cfg.VMGroups == nil {
		cfg.VMGroups = make(map[string]string)
	}
	if cfg.LLM == nil {
		cfg.LLM = &LLMConfig{
			BaseURL: "https://openrouter.ai/api/v1",
			Model:   "anthropic/claude-sonnet-4",
		}
	}
	return &cfg, nil
}

func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func CreateDefault(path string) (*Config, error) {
	home, _ := os.UserHomeDir()
	cloudInitDir := filepath.Join(home, ".passgo-web", "cloud-init")

	cfg := &Config{
		Listen:       ":8080",
		CloudInitDir: cloudInitDir,
		Username:     "admin",
		Password:     "admin",
	}
	if err := cfg.Save(path); err != nil {
		return nil, err
	}
	return cfg, nil
}
