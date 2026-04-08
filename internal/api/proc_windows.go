//go:build windows

package api

import "os/exec"

func setProcGroup(cmd *exec.Cmd) {
	// No process group support on Windows; no-op.
}

func killProcGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}
