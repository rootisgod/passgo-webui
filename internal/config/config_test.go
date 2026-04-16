package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// --- Save / Load round-trip ---

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	orig := &Config{
		Listen:       ":9090",
		CloudInitDir: "/tmp/ci",
		Username:     "alice",
		Password:     "$2a$10$abcdefghijklmnopqrstuv", // pretend-bcrypt
		TrustProxy:   true,
		Groups:       []string{"prod", "dev"},
		VMGroups:     map[string]string{"web-01": "prod", "test-01": "dev"},
		LLM: &LLMConfig{
			BaseURL: "https://api.example.com",
			Model:   "gpt-4",
		},
		Profiles: []Profile{{ID: "p1", Name: "dev", CPUs: 2, MemoryMB: 2048, DiskGB: 10}},
	}
	if err := orig.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Load fills in zero-value defaults for fields left empty — but we set
	// Listen and Username explicitly, so DeepEqual should hold on those.
	if got.Listen != orig.Listen {
		t.Errorf("Listen: got %q, want %q", got.Listen, orig.Listen)
	}
	if got.Username != orig.Username {
		t.Errorf("Username: got %q", got.Username)
	}
	if !reflect.DeepEqual(got.Groups, orig.Groups) {
		t.Errorf("Groups: got %v, want %v", got.Groups, orig.Groups)
	}
	if !reflect.DeepEqual(got.VMGroups, orig.VMGroups) {
		t.Errorf("VMGroups: got %v, want %v", got.VMGroups, orig.VMGroups)
	}
	if !reflect.DeepEqual(got.Profiles, orig.Profiles) {
		t.Errorf("Profiles: got %+v, want %+v", got.Profiles, orig.Profiles)
	}
	if got.LLM == nil || got.LLM.Model != "gpt-4" {
		t.Errorf("LLM: got %+v", got.LLM)
	}
	if !got.TrustProxy {
		t.Error("TrustProxy lost in round-trip")
	}
}

// --- Atomic save behavior ---

func TestSave_NoStrayTmpFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	c := &Config{Listen: ":8080", Username: "u", Password: "p"}
	if err := c.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		// After a successful save, only c.json should be present — no .config-*.tmp leftovers.
		if e.Name() != "c.json" && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("stray tmp file after save: %s", e.Name())
		}
	}
}

func TestSave_OverwritesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	v1 := &Config{Listen: ":1", Username: "v1user", Password: "x"}
	v2 := &Config{Listen: ":2", Username: "v2user", Password: "x"}

	if err := v1.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := v2.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "v2user" {
		t.Errorf("v2 not persisted: got %q", got.Username)
	}
}

func TestSave_UnwritableDirReturnsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	// A path whose parent dir doesn't exist and can't be created triggers MkdirAll error.
	// Use /proc (Linux) or /dev/null/ as invalid — /dev/null/x works on macOS + Linux.
	c := &Config{Listen: ":8080", Username: "u", Password: "p"}
	err := c.Save("/dev/null/cannot/be/created/c.json")
	if err == nil {
		t.Error("expected save error on invalid path")
	}
}

// --- Load defaults ---

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_FillsDefaults(t *testing.T) {
	// Write a minimal config and verify defaults are filled on Load.
	path := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Listen != ":8080" {
		t.Errorf("Listen default: got %q", c.Listen)
	}
	if c.Username != "admin" {
		t.Errorf("Username default: got %q", c.Username)
	}
	if c.Password != "admin" {
		t.Errorf("Password default: got %q", c.Password)
	}
	if c.Groups == nil {
		t.Error("Groups should default to empty slice, not nil")
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected parse error")
	}
}

// --- Password / bcrypt ---

func TestHashPassword_VerifiesWithBcrypt(t *testing.T) {
	hash, err := HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("s3cret")); err != nil {
		t.Errorf("bcrypt compare: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrong")); err == nil {
		t.Error("wrong password should not match")
	}
}

func TestMigratePassword_PlaintextGetsHashed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	cfg := &Config{Listen: ":8080", Username: "u", Password: "plain-text"}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigratePassword(cfg, path)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected migrated=true")
	}
	if !strings.HasPrefix(cfg.Password, "$2") {
		t.Errorf("password not bcrypt-hashed: %q", cfg.Password)
	}
	// The old plaintext must still authenticate via bcrypt.
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.Password), []byte("plain-text")); err != nil {
		t.Errorf("migrated hash does not match original password: %v", err)
	}
	// File on disk should also have the hashed version.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Password != cfg.Password {
		t.Error("migrated password not persisted to disk")
	}
}

func TestMigratePassword_AlreadyHashedNoOp(t *testing.T) {
	already, _ := HashPassword("existing")
	cfg := &Config{Password: already}
	migrated, err := MigratePassword(cfg, filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if migrated {
		t.Error("expected migrated=false for already-hashed password")
	}
	if cfg.Password != already {
		t.Error("hash should be unchanged")
	}
}

// --- Profile CRUD ---

func TestProfileCRUD(t *testing.T) {
	c := &Config{}
	p := Profile{ID: "dev", Name: "Dev", CPUs: 2, MemoryMB: 2048, DiskGB: 10}

	if err := c.AddProfile(p); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := c.AddProfile(p); err == nil {
		t.Error("duplicate ID should error")
	}

	got, idx := c.GetProfile("dev")
	if idx != 0 || got == nil || got.Name != "Dev" {
		t.Errorf("get: idx=%d got=%+v", idx, got)
	}

	p.Name = "DevUpdated"
	if err := c.UpdateProfile(p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = c.GetProfile("dev")
	if got.Name != "DevUpdated" {
		t.Errorf("after update: %q", got.Name)
	}

	if err := c.UpdateProfile(Profile{ID: "nope", Name: "x"}); err == nil {
		t.Error("update non-existent should error")
	}

	if err := c.DeleteProfile("dev"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, idx := c.GetProfile("dev"); idx != -1 {
		t.Error("still present after delete")
	}
	if err := c.DeleteProfile("dev"); err == nil {
		t.Error("delete non-existent should error")
	}
}

func TestProfileValidate(t *testing.T) {
	if err := (&Profile{}).Validate(); err == nil {
		t.Error("empty profile should fail validation")
	}
	if err := (&Profile{ID: "bad id!", Name: "x"}).Validate(); err == nil {
		t.Error("invalid ID chars should fail")
	}
	if err := (&Profile{ID: "ok", Name: "x", MemoryMB: 100}).Validate(); err == nil {
		t.Error("memory below 512 should fail")
	}
	if err := (&Profile{ID: "ok", Name: "x", MemoryMB: 0, CPUs: 0, DiskGB: 0}).Validate(); err != nil {
		t.Errorf("zero memory/cpu/disk (use defaults) should be valid: %v", err)
	}
}

// --- Schedule CRUD ---

func TestScheduleCRUD(t *testing.T) {
	c := &Config{}
	s := Schedule{
		ID: "morning", Name: "Morning start", Enabled: true,
		Action: "start", Time: "09:00", Days: []int{1, 2, 3, 4, 5},
		Group: "prod",
	}
	if err := c.AddSchedule(s); err != nil {
		t.Fatalf("add: %v", err)
	}
	got, _ := c.GetSchedule("morning")
	if got == nil || got.Time != "09:00" {
		t.Errorf("get: %+v", got)
	}

	s.Time = "08:00"
	if err := c.UpdateSchedule(s); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = c.GetSchedule("morning")
	if got.Time != "08:00" {
		t.Errorf("after update: %q", got.Time)
	}

	if err := c.DeleteSchedule("morning"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, idx := c.GetSchedule("morning"); idx != -1 {
		t.Error("still present after delete")
	}
}

func TestScheduleValidate(t *testing.T) {
	cases := []struct {
		name string
		s    Schedule
		ok   bool
	}{
		{"valid", Schedule{ID: "a", Name: "n", Action: "start", Time: "08:00", Days: []int{1}, Group: "g"}, true},
		{"empty id", Schedule{Name: "n", Action: "start", Time: "08:00", Days: []int{1}, Group: "g"}, false},
		{"bad action", Schedule{ID: "a", Name: "n", Action: "zap", Time: "08:00", Days: []int{1}, Group: "g"}, false},
		{"bad time", Schedule{ID: "a", Name: "n", Action: "start", Time: "25:00", Days: []int{1}, Group: "g"}, false},
		{"no days", Schedule{ID: "a", Name: "n", Action: "start", Time: "08:00", Group: "g"}, false},
		{"playbook action no playbook", Schedule{ID: "a", Name: "n", Action: "playbook", Time: "08:00", Days: []int{1}, Group: "g"}, false},
		{"no vms or group", Schedule{ID: "a", Name: "n", Action: "start", Time: "08:00", Days: []int{1}}, false},
	}
	for _, tc := range cases {
		err := tc.s.Validate()
		if tc.ok && err != nil {
			t.Errorf("%s: expected ok, got %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

// --- Webhook CRUD ---

func TestWebhookCRUD(t *testing.T) {
	c := &Config{}
	w := Webhook{ID: "w1", Name: "slack", URL: "https://hooks.example.com/x", Enabled: true}
	if err := c.AddWebhook(w); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got, _ := c.GetWebhook("w1"); got == nil {
		t.Fatal("not found after add")
	}
	w.Enabled = false
	if err := c.UpdateWebhook(w); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := c.GetWebhook("w1")
	if got.Enabled {
		t.Error("update didn't take")
	}
	if err := c.DeleteWebhook("w1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.DeleteWebhook("w1"); err == nil {
		t.Error("delete non-existent should error")
	}
}

func TestWebhookValidate(t *testing.T) {
	bad := []Webhook{
		{Name: "", URL: "https://ok"},
		{Name: "x", URL: ""},
		{Name: "x", URL: "ftp://nope"},
		{Name: "x", URL: "https://ok", Categories: []string{"made-up"}},
		{Name: "x", URL: "https://ok", Results: []string{"weird"}},
	}
	for _, w := range bad {
		if err := w.Validate(); err == nil {
			t.Errorf("expected validation error for %+v", w)
		}
	}

	good := Webhook{Name: "x", URL: "https://ok", Categories: []string{"vm", "schedule"}, Results: []string{"success", "failed"}}
	if err := good.Validate(); err != nil {
		t.Errorf("valid webhook rejected: %v", err)
	}
}

// --- APIToken CRUD ---

func TestAPITokenCRUD(t *testing.T) {
	c := &Config{}
	tok := APIToken{ID: "t1", Name: "ci", Prefix: "pgo_abc", Hash: "hash1", CreatedAt: "2026-04-16"}
	if err := c.AddAPIToken(tok); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got, _ := c.GetAPIToken("t1"); got == nil || got.Hash != "hash1" {
		t.Errorf("get: %+v", got)
	}
	if err := c.DeleteAPIToken("t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.DeleteAPIToken("t1"); err == nil {
		t.Error("delete non-existent should error")
	}
}

// --- CreateDefault ---

func TestCreateDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	cfg, err := CreateDefault(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if cfg.Username != "admin" {
		t.Errorf("username: %q", cfg.Username)
	}
	// Default password must be bcrypt-hashed, not literal "admin".
	if !strings.HasPrefix(cfg.Password, "$2") {
		t.Errorf("expected bcrypt password, got %q", cfg.Password)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.Password), []byte("admin")); err != nil {
		t.Errorf("default password doesn't verify: %v", err)
	}

	// The file should exist on disk.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
