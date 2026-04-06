//go:build darwin

package multipass

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GetHostResources returns the host machine's CPU count, load averages,
// memory usage, and disk usage on macOS.
func GetHostResources() (HostResources, error) {
	res := HostResources{
		TotalCPUs: runtime.NumCPU(),
	}

	// Total memory
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return res, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	memBytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return res, fmt.Errorf("parse hw.memsize: %w", err)
	}
	res.TotalMemoryMB = memBytes / (1024 * 1024)

	// Load averages
	if load1, load5, load15, err := parseLoadAvgDarwin(); err == nil {
		res.LoadAvg1 = load1
		res.LoadAvg5 = load5
		res.LoadAvg15 = load15
	}

	// Memory usage via vm_stat
	if used, err := parseMemUsageDarwin(); err == nil {
		res.UsedMemoryMB = used
	}

	// Disk usage via df
	if total, used, err := parseDiskUsage(); err == nil {
		res.TotalDiskMB = total
		res.UsedDiskMB = used
	}

	return res, nil
}

func parseLoadAvgDarwin() (float64, float64, float64, error) {
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, 0, 0, err
	}
	// Output format: "{ 0.45 0.52 0.48 }"
	s := strings.TrimSpace(string(out))
	s = strings.Trim(s, "{ }")
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected loadavg format: %s", s)
	}
	l1, err1 := strconv.ParseFloat(parts[0], 64)
	l5, err2 := strconv.ParseFloat(parts[1], 64)
	l15, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, fmt.Errorf("parse loadavg values")
	}
	return l1, l5, l15, nil
}

func parseMemUsageDarwin() (int64, error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, err
	}

	pageSize := int64(4096) // macOS default page size
	var active, wired, compressed int64

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Pages active:") {
			active = parseVmStatValue(line)
		} else if strings.HasPrefix(line, "Pages wired down:") {
			wired = parseVmStatValue(line)
		} else if strings.HasPrefix(line, "Pages occupied by compressor:") {
			compressed = parseVmStatValue(line)
		}
	}

	usedBytes := (active + wired + compressed) * pageSize
	return usedBytes / (1024 * 1024), nil
}

func parseVmStatValue(line string) int64 {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return 0
	}
	s := strings.TrimSpace(parts[1])
	s = strings.TrimSuffix(s, ".")
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func parseDiskUsage() (int64, int64, error) {
	// On macOS with APFS, "/" is a read-only system snapshot.
	// The actual user data volume is mounted at /System/Volumes/Data.
	// Try the Data volume first, fall back to "/".
	path := "/System/Volumes/Data"
	out, err := exec.Command("df", "-k", path).Output()
	if err != nil {
		path = "/"
		out, err = exec.Command("df", "-k", path).Output()
		if err != nil {
			return 0, 0, err
		}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, 0, fmt.Errorf("unexpected df output")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, 0, fmt.Errorf("unexpected df fields")
	}
	// df -k outputs 1K blocks: total is field[1], used is field[2]
	totalKB, err1 := strconv.ParseInt(fields[1], 10, 64)
	usedKB, err2 := strconv.ParseInt(fields[2], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("parse df values")
	}
	return totalKB / 1024, usedKB / 1024, nil
}
