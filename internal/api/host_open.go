package api

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openHostPath launches the host OS's native file manager at the given path.
// Uses Start (not Run) so the HTTP handler doesn't block waiting for the GUI.
// Returns an error only if the launcher binary isn't available — not for
// whatever exit code the launcher produces later, since e.g. Windows Explorer
// commonly exits non-zero on success.
func openHostPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		return fmt.Errorf("opening folders is not supported on %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch file manager: %w", err)
	}
	// Detach — we don't wait for exit. Release so we don't leak a zombie
	// on Unix systems.
	go func() { _ = cmd.Wait() }()
	return nil
}
