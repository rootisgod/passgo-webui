package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Listen        string `json:"listen"`
	Username      string `json:"username"`
	PasswordHash  string `json:"password_hash"`
	CloudInitDir  string `json:"cloud_init_dir"`
	CloudInitRepo string `json:"cloud_init_repo"`
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

// CreateDefault creates a new config with generated credentials, saves it, and returns
// the config and the plaintext password (for display to the user on first run).
func CreateDefault(path string) (*Config, string, error) {
	password := generatePassword()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	// Default cloud-init directory alongside config
	home, _ := os.UserHomeDir()
	cloudInitDir := filepath.Join(home, ".passgo-web", "cloud-init")

	cfg := &Config{
		Listen:       ":8080",
		Username:     "admin",
		PasswordHash: string(hash),
		CloudInitDir: cloudInitDir,
	}
	if err := cfg.Save(path); err != nil {
		return nil, "", err
	}
	return cfg, password, nil
}

func (c *Config) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(c.PasswordHash), []byte(password)) == nil
}

func generatePassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "changeme"
	}
	return hex.EncodeToString(b)[:16]
}
