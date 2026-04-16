package multipass

import (
	"fmt"
	"regexp"
)

const (
	DefaultUbuntuRelease = "24.04"
	DefaultCPUCores      = 2
	DefaultRAMMB         = 1024
	DefaultDiskGB        = 8
	MinCPUCores          = 1
	MinRAMMB             = 512
	MinResizeRAMMB       = 256
	MinDiskGB            = 1
	VMNamePrefix         = "VM-"
	VMNameRandomLength   = 4
)

var UbuntuReleases = []string{"24.04", "22.04", "20.04", "18.04", "daily"}

// vmNamePattern mirrors multipass's own instance-name rules: leading letter/digit,
// then up to 62 more letters, digits, or hyphens. Rejecting leading `-` is the
// important part — it blocks flag injection when names reach exec.Command argv.
var (
	vmNamePattern      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,62}$`)
	groupNamePattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 _-]{0,62}$`)
	profileIDPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)
	playbookFilePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}\.ya?ml$`)
)

func ValidateVMName(name string) error {
	if !vmNamePattern.MatchString(name) {
		return fmt.Errorf("invalid VM name %q: must start with letter/digit and contain only letters, digits, and hyphens (max 63 chars)", name)
	}
	return nil
}

func ValidateGroupName(name string) error {
	if !groupNamePattern.MatchString(name) {
		return fmt.Errorf("invalid group name %q: must start with letter/digit and contain only letters, digits, spaces, hyphens, and underscores (max 63 chars)", name)
	}
	return nil
}

func ValidateProfileID(id string) error {
	if !profileIDPattern.MatchString(id) {
		return fmt.Errorf("invalid profile id %q: must start with letter/digit and contain only letters, digits, hyphens, and underscores (max 63 chars)", id)
	}
	return nil
}

func ValidatePlaybookFilename(name string) error {
	if !playbookFilePattern.MatchString(name) {
		return fmt.Errorf("invalid playbook filename %q: must end in .yml or .yaml and contain only letters, digits, hyphens, and underscores", name)
	}
	return nil
}
