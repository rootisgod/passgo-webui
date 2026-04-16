package multipass

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSanitizePlaybookName_Reject(t *testing.T) {
	base := t.TempDir()
	bad := []string{
		"",
		"play",
		"../evil.yml",
		"a/b.yml",
		"a\\b.yml",
		"..yml",
		"bad$.yml",
	}
	for _, n := range bad {
		if _, err := sanitizePlaybookName(base, n); err == nil {
			t.Errorf("sanitizePlaybookName(%q) = nil, want error", n)
		}
	}
}

func TestSanitizePlaybookName_Accept(t *testing.T) {
	base := t.TempDir()
	good := []string{"deploy.yml", "setup.yaml", "my-play_01.yml"}
	for _, n := range good {
		if _, err := sanitizePlaybookName(base, n); err != nil {
			t.Errorf("sanitizePlaybookName(%q) rejected: %v", n, err)
		}
	}
}

func TestPlaybook_RoundTrip(t *testing.T) {
	base := t.TempDir()
	const name = "site.yml"
	const content = "- hosts: all\n  tasks:\n    - debug: msg=hi\n"

	if err := WritePlaybook(base, name, content); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadPlaybook(base, name)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != content {
		t.Errorf("content mismatch")
	}

	if err := DeletePlaybook(base, name); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, name)); !os.IsNotExist(err) {
		t.Error("file still exists after delete")
	}
}

func TestListPlaybooks_FiltersAndSorts(t *testing.T) {
	base := t.TempDir()
	// Mixed files — only .yml/.yaml should appear, sorted alphabetically.
	files := []string{"zebra.yml", "alpha.yaml", "readme.txt", "beta.YML", "config.json"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(base, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// And a directory that must be skipped
	if err := os.Mkdir(filepath.Join(base, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := ListPlaybooks(base)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"alpha.yaml", "beta.YML", "zebra.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("list: got %v, want %v", got, want)
	}
}

func TestListPlaybooks_MissingDir(t *testing.T) {
	got, err := ListPlaybooks("/no/such/dir/exists/here")
	if err != nil {
		t.Fatalf("want nil error for missing dir, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

func TestDeletePlaybook_NotFound(t *testing.T) {
	base := t.TempDir()
	if err := DeletePlaybook(base, "missing.yml"); err == nil {
		t.Error("expected not-found error")
	}
}
