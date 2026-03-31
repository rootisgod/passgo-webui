package multipass

import "os/exec"

// newGitCommand creates an exec.Command for git (used by cloud-init repo cloning).
func newGitCommand(args ...string) *exec.Cmd {
	return exec.Command("git", args...)
}
