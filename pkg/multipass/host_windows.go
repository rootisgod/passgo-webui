//go:build windows

package multipass

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GetHostResources returns the host machine's CPU count, memory usage,
// and disk usage on Windows. Load averages are not available on Windows.
func GetHostResources() (HostResources, error) {
	res := HostResources{
		TotalCPUs: runtime.NumCPU(),
	}

	// Memory via wmic
	if total, free, err := parseMemWindows(); err == nil {
		res.TotalMemoryMB = total / 1024 // wmic returns KB
		res.UsedMemoryMB = (total - free) / 1024
	}

	// Disk via wmic
	if total, free, err := parseDiskWindows(); err == nil {
		res.TotalDiskMB = total / (1024 * 1024) // wmic returns bytes
		res.UsedDiskMB = (total - free) / (1024 * 1024)
	}

	return res, nil
}

func parseMemWindows() (int64, int64, error) {
	out, err := exec.Command("wmic", "OS", "get", "TotalVisibleMemorySize,FreePhysicalMemory", "/value").Output()
	if err != nil {
		return 0, 0, err
	}
	var total, free int64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TotalVisibleMemorySize=") {
			v := strings.TrimPrefix(line, "TotalVisibleMemorySize=")
			total, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		} else if strings.HasPrefix(line, "FreePhysicalMemory=") {
			v := strings.TrimPrefix(line, "FreePhysicalMemory=")
			free, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("failed to parse memory info")
	}
	return total, free, nil
}

func parseDiskWindows() (int64, int64, error) {
	out, err := exec.Command("wmic", "LogicalDisk", "where", "DeviceID='C:'", "get", "Size,FreeSpace", "/value").Output()
	if err != nil {
		return 0, 0, err
	}
	var size, free int64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Size=") {
			v := strings.TrimPrefix(line, "Size=")
			size, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		} else if strings.HasPrefix(line, "FreeSpace=") {
			v := strings.TrimPrefix(line, "FreeSpace=")
			free, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		}
	}
	if size == 0 {
		return 0, 0, fmt.Errorf("failed to parse disk info")
	}
	return size, free, nil
}
