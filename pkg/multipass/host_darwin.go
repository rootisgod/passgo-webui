//go:build darwin

package multipass

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GetHostResources returns the host machine's CPU count and total RAM on macOS.
func GetHostResources() (HostResources, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return HostResources{}, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	memBytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return HostResources{}, fmt.Errorf("parse hw.memsize: %w", err)
	}
	return HostResources{
		TotalMemoryMB: memBytes / (1024 * 1024),
		TotalCPUs:     runtime.NumCPU(),
	}, nil
}
