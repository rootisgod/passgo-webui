package multipass

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestSanitizeTemplateName_Reject(t *testing.T) {
	base := t.TempDir()
	bad := []string{
		"",                 // empty
		"foo",              // no extension
		"../evil.yml",      // traversal
		"foo/bar.yml",      // slash
		"foo\\bar.yml",     // backslash
		"..yml",            // traversal substring + invalid
		"foo.yml\x00evil",  // null byte
		"foo bar.yml",      // space
		"foo$.yml",         // special char
	}
	for _, name := range bad {
		if _, err := sanitizeTemplateName(base, name); err == nil {
			t.Errorf("sanitizeTemplateName(%q) = nil, want error", name)
		}
	}
}

func TestSanitizeTemplateName_Accept(t *testing.T) {
	base := t.TempDir()
	good := []string{"a.yml", "deploy.yaml", "my-template.yml", "with_under.yml", "Name01.YML"}
	for _, name := range good {
		p, err := sanitizeTemplateName(base, name)
		if err != nil {
			t.Errorf("sanitizeTemplateName(%q) unexpected error: %v", name, err)
			continue
		}
		if !strings.HasPrefix(p, base) {
			t.Errorf("path escaped base dir: %s", p)
		}
	}
}

func TestCloudInitTemplate_WriteReadDeleteRoundTrip(t *testing.T) {
	base := t.TempDir()
	const name = "deploy.yml"
	const content = "#cloud-config\nruncmd:\n  - echo hi\n"

	if err := WriteCloudInitTemplate(base, name, content); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadCloudInitTemplate(base, name)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", got, content)
	}

	// Verify the file is actually inside base
	if _, err := os.Stat(filepath.Join(base, name)); err != nil {
		t.Errorf("file not at expected path: %v", err)
	}

	if err := DeleteCloudInitTemplate(base, name); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, name)); !os.IsNotExist(err) {
		t.Errorf("file still exists after delete")
	}
}

func TestDeleteCloudInitTemplate_NotFound(t *testing.T) {
	base := t.TempDir()
	if err := DeleteCloudInitTemplate(base, "missing.yml"); err == nil {
		t.Error("expected not-found error")
	}
}

func TestReadCloudInitTemplate_RejectsTraversal(t *testing.T) {
	base := t.TempDir()
	// Create a file outside base the test would never want to read.
	outside := filepath.Join(filepath.Dir(base), "secret.yml")
	if err := os.WriteFile(outside, []byte("nope"), 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)

	if _, err := ReadCloudInitTemplate(base, "../secret.yml"); err == nil {
		t.Error("expected traversal rejection")
	}
}

func TestWriteCloudInitTemplate_RejectsTraversal(t *testing.T) {
	base := t.TempDir()
	if err := WriteCloudInitTemplate(base, "../evil.yml", "x"); err == nil {
		t.Error("expected traversal rejection")
	}
}

func TestValidateCloudInitYAML_Valid(t *testing.T) {
	valid := "#cloud-config\npackages:\n  - nginx\n"
	if err := ValidateCloudInitYAML(valid); err != nil {
		t.Errorf("valid YAML rejected: %v", err)
	}
}

func TestValidateCloudInitYAML_Invalid(t *testing.T) {
	invalid := "not: valid: yaml: at: all"
	if err := ValidateCloudInitYAML(invalid); err == nil {
		t.Error("expected error for malformed YAML")
	}
}

// Helper used by playbook tests too: ListPlaybooks filter semantics are the
// same shape as a hypothetical ListCloudInitTemplates — but that function
// doesn't exist, so skip. ScanCloudInitTemplates is tested below.

func TestScanCloudInitTemplates_FiltersByContent(t *testing.T) {
	base := t.TempDir()
	// A valid cloud-init file (has #cloud-config header)
	if err := os.WriteFile(filepath.Join(base, "good.yml"), []byte("#cloud-config\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// A YAML file without the header — should be skipped
	if err := os.WriteFile(filepath.Join(base, "plain.yml"), []byte("just: yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// A non-YAML file — should be skipped
	if err := os.WriteFile(filepath.Join(base, "readme.txt"), []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewClientWithRunner(discardLogger(), nil)
	opts, err := c.ScanCloudInitTemplates([]string{base})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	var labels []string
	for _, o := range opts {
		labels = append(labels, o.Label)
	}
	sort.Strings(labels)
	want := []string{"good.yml"}
	if !reflect.DeepEqual(labels, want) {
		t.Errorf("scanned: got %v, want %v", labels, want)
	}
}
