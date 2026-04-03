//go:build windows

package multipass

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GetHostResources returns the host machine's CPU count and total RAM on Windows.
func GetHostResources() (HostResources, error) {
	out, err := exec.Command("wmic", "ComputerSystem", "get", "TotalPhysicalMemory", "/value").Output()
	if err != nil {
		return HostResources{}, fmt.Errorf("wmic TotalPhysicalMemory: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TotalPhysicalMemory=") {
			valStr := strings.TrimPrefix(line, "TotalPhysicalMemory=")
			memBytes, err := strconv.ParseInt(strings.TrimSpace(valStr), 10, 64)
			if err != nil {
				return HostResources{}, fmt.Errorf("parse TotalPhysicalMemory: %w", err)
			}
			return HostResources{
				TotalMemoryMB: memBytes / (1024 * 1024),
				TotalCPUs:     runtime.NumCPU(),
			}, nil
		}
	}
	return HostResources{}, fmt.Errorf("TotalPhysicalMemory not found in wmic output")
}
