package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/aiaccess"
	"golang.org/x/crypto/bcrypt"
)

var profileIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type LLMConfig struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key,omitempty"`
	Model    string `json:"model"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Provider string `json:"provider,omitempty"`
}

func DefaultLLMConfig() *LLMConfig {
	defaults := aiaccess.DefaultConfig()
	return &LLMConfig{
		BaseURL:  defaults.BaseURL,
		Model:    defaults.Model,
		Provider: aiaccess.ProviderVercelGateway,
	}
}

type VMDefaults struct {
	CPUs          int    `json:"cpus"`
	MemoryMB      int    `json:"memory_mb"`
	DiskGB        int    `json:"disk_gb"`
	SSHPublicKey  string `json:"ssh_public_key,omitempty"`
	SSHPrivateKey string `json:"ssh_private_key,omitempty"`
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

var agentReadyProfile = Profile{
	ID:        "agent-ready",
	Name:      "Agent-ready Ubuntu",
	Release:   "24.04",
	CPUs:      2,
	MemoryMB:  4096,
	DiskGB:    20,
	CloudInit: "builtin:agent-ready.yml",
	Group:     "agents",
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

type APIToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"created_at"`
}

type Webhook struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Enabled    bool     `json:"enabled"`
	Categories []string `json:"categories,omitempty"`
	Results    []string `json:"results,omitempty"`
	Secret     string   `json:"secret,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

var validWebhookCategories = map[string]bool{
	"vm": true, "schedule": true, "ansible": true, "llm": true, "config": true,
}

var validWebhookResults = map[string]bool{
	"success": true, "failed": true, "partial": true, "no_targets": true,
}

func (w *Webhook) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("name is required")
	}
	if w.URL == "" {
		return fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(w.URL, "http://") && !strings.HasPrefix(w.URL, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}
	for _, c := range w.Categories {
		if !validWebhookCategories[c] {
			return fmt.Errorf("invalid category %q", c)
		}
	}
	for _, r := range w.Results {
		if !validWebhookResults[r] {
			return fmt.Errorf("invalid result filter %q", r)
		}
	}
	return nil
}

type ProxyRule struct {
	ID                string `json:"id"`
	VM                string `json:"vm"`
	Port              int    `json:"port"`
	Protocol          string `json:"protocol,omitempty"`
	HostPort          int    `json:"host_port,omitempty"`
	BindAddress       string `json:"bind_address,omitempty"`
	Label             string `json:"label,omitempty"`
	Owner             string `json:"owner,omitempty"`
	Enabled           bool   `json:"enabled"`
	ExpiresAt         string `json:"expires_at,omitempty"`
	CreatedAt         string `json:"created_at"`
	AccessTokenHash   string `json:"access_token_hash,omitempty"`
	AccessTokenPrefix string `json:"access_token_prefix,omitempty"`
	LastAccessedAt    string `json:"last_accessed_at,omitempty"`
	AccessCount       int    `json:"access_count,omitempty"`
}

func (p *ProxyRule) Validate() error {
	p.Protocol = normalizeProxyProtocol(p.Protocol)
	if p.ID == "" {
		return fmt.Errorf("id is required")
	}
	if !profileIDRegex.MatchString(p.ID) {
		return fmt.Errorf("id must contain only letters, numbers, hyphens, and underscores")
	}
	if p.VM == "" {
		return fmt.Errorf("vm is required")
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if p.Protocol != "http" && p.Protocol != "ssh" {
		return fmt.Errorf("protocol must be http or ssh")
	}
	if p.Protocol == "http" {
		if p.HostPort != 0 {
			return fmt.Errorf("host_port is only valid for ssh proxy rules")
		}
		if strings.TrimSpace(p.BindAddress) != "" {
			return fmt.Errorf("bind_address is only valid for ssh proxy rules")
		}
		if p.AccessTokenHash != "" {
			if len(p.AccessTokenHash) != 64 {
				return fmt.Errorf("access_token_hash must be a SHA-256 hex digest")
			}
			if _, err := hex.DecodeString(p.AccessTokenHash); err != nil {
				return fmt.Errorf("access_token_hash must be a SHA-256 hex digest")
			}
		}
	}
	if p.Protocol == "ssh" {
		if p.HostPort < 1 || p.HostPort > 65535 {
			return fmt.Errorf("host_port must be between 1 and 65535 for ssh proxy rules")
		}
		p.BindAddress = normalizeProxyBindAddress(p.BindAddress)
		if err := validateProxyBindAddress(p.BindAddress); err != nil {
			return err
		}
		if p.AccessTokenHash != "" || p.AccessTokenPrefix != "" {
			return fmt.Errorf("access tokens are only valid for http proxy rules")
		}
	}
	if len(p.Label) > 80 {
		return fmt.Errorf("label must be 80 characters or less")
	}
	if len(p.Owner) > 80 {
		return fmt.Errorf("owner must be 80 characters or less")
	}
	if p.ExpiresAt != "" {
		if _, err := time.Parse(time.RFC3339, p.ExpiresAt); err != nil {
			return fmt.Errorf("expires_at must be RFC3339")
		}
	}
	if p.LastAccessedAt != "" {
		if _, err := time.Parse(time.RFC3339, p.LastAccessedAt); err != nil {
			return fmt.Errorf("last_accessed_at must be RFC3339")
		}
	}
	if p.AccessCount < 0 {
		return fmt.Errorf("access_count must not be negative")
	}
	return nil
}

func normalizeProxyProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "http", "http/ws", "web", "websocket":
		return "http"
	case "ssh", "tcp":
		return "ssh"
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
}

func normalizeProxyBindAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return "0.0.0.0"
	}
	switch strings.ToLower(address) {
	case "all", "lan":
		return "0.0.0.0"
	case "local", "localhost":
		return "127.0.0.1"
	default:
		return address
	}
}

func validateProxyBindAddress(address string) error {
	if address == "" {
		return nil
	}
	if strings.Contains(address, "/") {
		return fmt.Errorf("bind_address must be an IP address, not a CIDR")
	}
	ip := net.ParseIP(address)
	if ip == nil {
		return fmt.Errorf("bind_address must be an IP address")
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
	VMTemplates   map[string]bool   `json:"vm_templates,omitempty"`
	LLM           *LLMConfig        `json:"llm,omitempty"`
	VMDefaults    *VMDefaults       `json:"vm_defaults,omitempty"`
	PlaybooksDir  string            `json:"playbooks_dir,omitempty"`
	Profiles      []Profile         `json:"profiles,omitempty"`
	Schedules     []Schedule        `json:"schedules,omitempty"`
	APITokens     []APIToken        `json:"api_tokens,omitempty"`
	Webhooks      []Webhook         `json:"webhooks,omitempty"`
	ProxyRules    []ProxyRule       `json:"proxy_rules,omitempty"`
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

func (c *Config) EnsureBuiltInProfiles() {
	if existing, _ := c.GetProfile(agentReadyProfile.ID); existing != nil {
		return
	}
	c.Profiles = append(c.Profiles, agentReadyProfile)
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

func (c *Config) GetAPITokens() []APIToken {
	if c.APITokens == nil {
		return []APIToken{}
	}
	return c.APITokens
}

func (c *Config) GetAPIToken(id string) (*APIToken, int) {
	for i := range c.APITokens {
		if c.APITokens[i].ID == id {
			return &c.APITokens[i], i
		}
	}
	return nil, -1
}

func (c *Config) AddAPIToken(t APIToken) error {
	c.APITokens = append(c.APITokens, t)
	return nil
}

func (c *Config) DeleteAPIToken(id string) error {
	_, idx := c.GetAPIToken(id)
	if idx == -1 {
		return fmt.Errorf("api token %q not found", id)
	}
	c.APITokens = append(c.APITokens[:idx], c.APITokens[idx+1:]...)
	return nil
}

func (c *Config) GetWebhooks() []Webhook {
	if c.Webhooks == nil {
		return []Webhook{}
	}
	return c.Webhooks
}

func (c *Config) GetWebhook(id string) (*Webhook, int) {
	for i := range c.Webhooks {
		if c.Webhooks[i].ID == id {
			return &c.Webhooks[i], i
		}
	}
	return nil, -1
}

func (c *Config) AddWebhook(w Webhook) error {
	if err := w.Validate(); err != nil {
		return err
	}
	if existing, _ := c.GetWebhook(w.ID); existing != nil {
		return fmt.Errorf("webhook with id %q already exists", w.ID)
	}
	c.Webhooks = append(c.Webhooks, w)
	return nil
}

func (c *Config) UpdateWebhook(w Webhook) error {
	if err := w.Validate(); err != nil {
		return err
	}
	_, idx := c.GetWebhook(w.ID)
	if idx == -1 {
		return fmt.Errorf("webhook %q not found", w.ID)
	}
	c.Webhooks[idx] = w
	return nil
}

func (c *Config) DeleteWebhook(id string) error {
	_, idx := c.GetWebhook(id)
	if idx == -1 {
		return fmt.Errorf("webhook %q not found", id)
	}
	c.Webhooks = append(c.Webhooks[:idx], c.Webhooks[idx+1:]...)
	return nil
}

func (c *Config) GetProxyRules() []ProxyRule {
	if c.ProxyRules == nil {
		return []ProxyRule{}
	}
	return c.ProxyRules
}

func (c *Config) GetProxyRule(id string) (*ProxyRule, int) {
	for i := range c.ProxyRules {
		if c.ProxyRules[i].ID == id {
			return &c.ProxyRules[i], i
		}
	}
	return nil, -1
}

func (c *Config) AddProxyRule(rule ProxyRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	if existing, _ := c.GetProxyRule(rule.ID); existing != nil {
		return fmt.Errorf("proxy rule with id %q already exists", rule.ID)
	}
	for _, existing := range c.ProxyRules {
		existing.Protocol = normalizeProxyProtocol(existing.Protocol)
		if existing.Protocol == "ssh" {
			existing.BindAddress = normalizeProxyBindAddress(existing.BindAddress)
		}
		if existing.Protocol == rule.Protocol && existing.VM == rule.VM && existing.Port == rule.Port {
			return fmt.Errorf("%s proxy rule for %s:%d already exists", rule.Protocol, rule.VM, rule.Port)
		}
		if rule.Protocol == "ssh" && existing.Protocol == "ssh" && existing.HostPort == rule.HostPort {
			return fmt.Errorf("ssh proxy rule for host port %d already exists", rule.HostPort)
		}
	}
	c.ProxyRules = append(c.ProxyRules, rule)
	return nil
}

func (c *Config) UpdateProxyRule(rule ProxyRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	_, idx := c.GetProxyRule(rule.ID)
	if idx == -1 {
		return fmt.Errorf("proxy rule %q not found", rule.ID)
	}
	for i, existing := range c.ProxyRules {
		existing.Protocol = normalizeProxyProtocol(existing.Protocol)
		if existing.Protocol == "ssh" {
			existing.BindAddress = normalizeProxyBindAddress(existing.BindAddress)
		}
		if i != idx && existing.Protocol == rule.Protocol && existing.VM == rule.VM && existing.Port == rule.Port {
			return fmt.Errorf("%s proxy rule for %s:%d already exists", rule.Protocol, rule.VM, rule.Port)
		}
		if i != idx && rule.Protocol == "ssh" && existing.Protocol == "ssh" && existing.HostPort == rule.HostPort {
			return fmt.Errorf("ssh proxy rule for host port %d already exists", rule.HostPort)
		}
	}
	c.ProxyRules[idx] = rule
	return nil
}

func (c *Config) DeleteProxyRule(id string) error {
	_, idx := c.GetProxyRule(id)
	if idx == -1 {
		return fmt.Errorf("proxy rule %q not found", id)
	}
	c.ProxyRules = append(c.ProxyRules[:idx], c.ProxyRules[idx+1:]...)
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
	if cfg.VMTemplates == nil {
		cfg.VMTemplates = make(map[string]bool)
	}
	if cfg.ProxyRules == nil {
		cfg.ProxyRules = []ProxyRule{}
	}
	for i := range cfg.ProxyRules {
		cfg.ProxyRules[i].Protocol = normalizeProxyProtocol(cfg.ProxyRules[i].Protocol)
		if cfg.ProxyRules[i].Protocol == "ssh" {
			cfg.ProxyRules[i].BindAddress = normalizeProxyBindAddress(cfg.ProxyRules[i].BindAddress)
		}
	}
	if cfg.LLM == nil {
		cfg.LLM = DefaultLLMConfig()
	}
	if cfg.PlaybooksDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.PlaybooksDir = filepath.Join(home, ".passgo-web", "playbooks")
		}
	}
	cfg.EnsureBuiltInProfiles()
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
	// Atomic write: write to tmp in the same directory, fsync, rename.
	// Same directory matters — os.Rename is only atomic within a filesystem,
	// and putting the tmp file elsewhere can fail across mount boundaries.
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create tmp config: %w", err)
	}
	tmpName := tmp.Name()
	// On any failure below, make sure the tmp file is removed.
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write tmp config: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("chmod tmp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("fsync tmp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close tmp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename tmp config: %w", err)
	}
	return nil
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
		Groups:       []string{},
		VMGroups:     make(map[string]string),
		VMTemplates:  make(map[string]bool),
		LLM:          DefaultLLMConfig(),
		Profiles:     []Profile{agentReadyProfile},
		ProxyRules:   []ProxyRule{},
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
