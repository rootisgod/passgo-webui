package multipass

import "testing"

func TestValidateVMName(t *testing.T) {
	good := []string{"a", "my-vm", "VM-abcd", "test-01", "Ubuntu-24-04"}
	for _, name := range good {
		if err := ValidateVMName(name); err != nil {
			t.Errorf("ValidateVMName(%q) = %v, want nil", name, err)
		}
	}

	bad := []string{
		"",          // empty
		"--all",     // flag injection
		"-name",     // leading hyphen
		"foo/bar",   // slash
		"foo..bar",  // path traversal fragment
		"foo bar",   // space
		"foo\x00b",  // null byte
		string(make([]byte, 100)), // too long + all zero bytes
	}
	for _, name := range bad {
		if err := ValidateVMName(name); err == nil {
			t.Errorf("ValidateVMName(%q) = nil, want error", name)
		}
	}
}

func TestValidateGroupName(t *testing.T) {
	if err := ValidateGroupName("my group"); err != nil {
		t.Errorf("group names with spaces should be valid: %v", err)
	}
	if err := ValidateGroupName("--all"); err == nil {
		t.Error("group name --all should be rejected")
	}
}

func TestValidatePlaybookFilename(t *testing.T) {
	if err := ValidatePlaybookFilename("deploy.yml"); err != nil {
		t.Errorf("deploy.yml rejected: %v", err)
	}
	if err := ValidatePlaybookFilename("deploy.yaml"); err != nil {
		t.Errorf("deploy.yaml rejected: %v", err)
	}
	if err := ValidatePlaybookFilename("deploy"); err == nil {
		t.Error("missing extension should be rejected")
	}
	if err := ValidatePlaybookFilename("../evil.yml"); err == nil {
		t.Error("traversal should be rejected")
	}
}
