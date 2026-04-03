//go:build linux

package multipass

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// GetHostResources returns the host machine's CPU count and total RAM on Linux.
func GetHostResources() (HostResources, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return HostResources{}, fmt.Errorf("open /proc/meminfo: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return HostResources{}, fmt.Errorf("unexpected MemTotal format: %s", line)
			}
			kB, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return HostResources{}, fmt.Errorf("parse MemTotal: %w", err)
			}
			return HostResources{
				TotalMemoryMB: kB / 1024,
				TotalCPUs:     runtime.NumCPU(),
			}, nil
		}
	}
	return HostResources{}, fmt.Errorf("MemTotal not found in /proc/meminfo")
}
