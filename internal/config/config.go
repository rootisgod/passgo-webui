package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var profileIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type LLMConfig struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key,omitempty"`
	Model    string `json:"model"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type VMDefaults struct {
	CPUs           int    `json:"cpus"`
	MemoryMB       int    `json:"memory_mb"`
	DiskGB         int    `json:"disk_gb"`
	SSHPublicKey   string `json:"ssh_public_key,omitempty"`
	SSHPrivateKey  string `json:"ssh_private_key,omitempty"`
}

type Profile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Release   string `json:"release,omitempty"`
	CPUs      int    `json:"cpus,omitempty"`
	MemoryMB  int    `json:"memory_mb,omitempty"`
	DiskGB    int    `json:"disk_gb,omitempty"`
	CloudInit string `json:"cloud_init,omitempty"`
	Network   string `json:"network,omitempty"`
	Playbook  string `json:"playbook,omitempty"`
	Group     string `json:"group,omitempty"`
}

func (p *Profile) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("id is required")
	}
	if !profileIDRegex.MatchString(p.ID) {
		return fmt.Errorf("id must contain only letters, numbers, hyphens, and underscores")
	}
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if p.CPUs < 0 || (p.CPUs > 0 && p.CPUs < 1) {
		return fmt.Errorf("cpus must be 0 (use default) or at least 1")
	}
	if p.MemoryMB < 0 || (p.MemoryMB > 0 && p.MemoryMB < 512) {
		return fmt.Errorf("memory_mb must be 0 (use default) or at least 512")
	}
	if p.DiskGB < 0 || (p.DiskGB > 0 && p.DiskGB < 1) {
		return fmt.Errorf("disk_gb must be 0 (use default) or at least 1")
	}
	return nil
}

type Schedule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	Action   string   `json:"action"`
	Time     string   `json:"time"`
	Days     []int    `json:"days"`
	VMs      []string `json:"vms,omitempty"`
	Group    string   `json:"group,omitempty"`
	Playbook string   `json:"playbook,omitempty"`
}

func (s *Schedule) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("id is required")
	}
	if !profileIDRegex.MatchString(s.ID) {
		return fmt.Errorf("id must contain only letters, numbers, hyphens, and underscores")
	}
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch s.Action {
	case "start", "stop", "playbook":
	default:
		return fmt.Errorf("action must be start, stop, or playbook")
	}
	// Validate time format HH:MM
	parts := strings.Split(s.Time, ":")
	if len(parts) != 2 {
		return fmt.Errorf("time must be in HH:MM format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return fmt.Errorf("hour must be 0-23")
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil || min < 0 || min > 59 {
		return fmt.Errorf("minute must be 0-59")
	}
	if len(s.Days) == 0 {
		return fmt.Errorf("at least one day is required")
	}
	for _, d := range s.Days {
		if d < 0 || d > 6 {
			return fmt.Errorf("day must be 0 (Sun) through 6 (Sat)")
		}
	}
	if len(s.VMs) == 0 && s.Group == "" {
		return fmt.Errorf("either vms or group must be specified")
	}
	if s.Action == "playbook" && s.Playbook == "" {
		return fmt.Errorf("playbook is required for playbook action")
	}
	return nil
}

type Config struct {
	Listen        string            `json:"listen"`
	CloudInitDir  string            `json:"cloud_init_dir"`
	CloudInitRepo string            `json:"cloud_init_repo"`
	Username      string            `json:"username"`
	Password      string            `json:"password"`
	TrustProxy    bool              `json:"trust_proxy,omitempty"`
	Groups        []string          `json:"groups,omitempty"`
	VMGroups      map[string]string `json:"vm_groups,omitempty"`
	LLM           *LLMConfig        `json:"llm,omitempty"`
	VMDefaults    *VMDefaults       `json:"vm_defaults,omitempty"`
	PlaybooksDir  string            `json:"playbooks_dir,omitempty"`
	Profiles      []Profile         `json:"profiles,omitempty"`
	Schedules     []Schedule        `json:"schedules,omitempty"`
}

func (c *Config) GetProfiles() []Profile {
	if c.Profiles == nil {
		return []Profile{}
	}
	return c.Profiles
}

func (c *Config) GetProfile(id string) (*Profile, int) {
	for i := range c.Profiles {
		if c.Profiles[i].ID == id {
			return &c.Profiles[i], i
		}
	}
	return nil, -1
}

func (c *Config) AddProfile(p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if existing, _ := c.GetProfile(p.ID); existing != nil {
		return fmt.Errorf("profile with id %q already exists", p.ID)
	}
	c.Profiles = append(c.Profiles, p)
	return nil
}

func (c *Config) UpdateProfile(p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, idx := c.GetProfile(p.ID)
	if idx == -1 {
		return fmt.Errorf("profile %q not found", p.ID)
	}
	c.Profiles[idx] = p
	return nil
}

func (c *Config) DeleteProfile(id string) error {
	_, idx := c.GetProfile(id)
	if idx == -1 {
		return fmt.Errorf("profile %q not found", id)
	}
	c.Profiles = append(c.Profiles[:idx], c.Profiles[idx+1:]...)
	return nil
}

func (c *Config) GetSchedules() []Schedule {
	if c.Schedules == nil {
		return []Schedule{}
	}
	return c.Schedules
}

func (c *Config) GetSchedule(id string) (*Schedule, int) {
	for i := range c.Schedules {
		if c.Schedules[i].ID == id {
			return &c.Schedules[i], i
		}
	}
	return nil, -1
}

func (c *Config) AddSchedule(s Schedule) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if existing, _ := c.GetSchedule(s.ID); existing != nil {
		return fmt.Errorf("schedule with id %q already exists", s.ID)
	}
	c.Schedules = append(c.Schedules, s)
	return nil
}

func (c *Config) UpdateSchedule(s Schedule) error {
	if err := s.Validate(); err != nil {
		return err
	}
	_, idx := c.GetSchedule(s.ID)
	if idx == -1 {
		return fmt.Errorf("schedule %q not found", s.ID)
	}
	c.Schedules[idx] = s
	return nil
}

func (c *Config) DeleteSchedule(id string) error {
	_, idx := c.GetSchedule(id)
	if idx == -1 {
		return fmt.Errorf("schedule %q not found", id)
	}
	c.Schedules = append(c.Schedules[:idx], c.Schedules[idx+1:]...)
	return nil
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
	if cfg.PlaybooksDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.PlaybooksDir = filepath.Join(home, ".passgo-web", "playbooks")
		}
	}
	if cfg.VMDefaults == nil {
		cfg.VMDefaults = &VMDefaults{CPUs: 2, MemoryMB: 1024, DiskGB: 8}
	}
	// Enforce minimums
	if cfg.VMDefaults.CPUs < 1 {
		cfg.VMDefaults.CPUs = 2
	}
	if cfg.VMDefaults.MemoryMB < 512 {
		cfg.VMDefaults.MemoryMB = 1024
	}
	if cfg.VMDefaults.DiskGB < 1 {
		cfg.VMDefaults.DiskGB = 8
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

	hashed, err := HashPassword("admin")
	if err != nil {
		return nil, fmt.Errorf("hash default password: %w", err)
	}

	cfg := &Config{
		Listen:       ":8080",
		CloudInitDir: cloudInitDir,
		Username:     "admin",
		Password:     hashed,
	}
	if err := cfg.Save(path); err != nil {
		return nil, err
	}
	return cfg, nil
}

// HashPassword returns the bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// MigratePassword checks if the stored password is plaintext (not bcrypt-hashed)
// and hashes it in place, saving the config. Call on startup to auto-migrate.
// After migration, plaintext comparison is no longer supported — only bcrypt.
func MigratePassword(cfg *Config, configPath string) (migrated bool, err error) {
	if len(cfg.Password) > 0 && cfg.Password[0] != '$' {
		hashed, err := HashPassword(cfg.Password)
		if err != nil {
			return false, err
		}
		cfg.Password = hashed
		return true, cfg.Save(configPath)
	}
	return false, nil
}
