//go:build linux

package multipass

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GetHostResources returns the host machine's CPU count, load averages,
// memory usage, and disk usage on Linux.
func GetHostResources() (HostResources, error) {
	res := HostResources{
		TotalCPUs: runtime.NumCPU(),
	}

	// Load averages from /proc/loadavg
	if load1, load5, load15, err := parseLoadAvgLinux(); err == nil {
		res.LoadAvg1 = load1
		res.LoadAvg5 = load5
		res.LoadAvg15 = load15
	}

	// Memory from /proc/meminfo
	if total, used, err := parseMemInfoLinux(); err == nil {
		res.TotalMemoryMB = total
		res.UsedMemoryMB = used
	}

	// Disk usage via df
	if total, used, err := parseDiskUsageLinux(); err == nil {
		res.TotalDiskMB = total
		res.UsedDiskMB = used
	}

	return res, nil
}

func parseLoadAvgLinux() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}
	l1, err1 := strconv.ParseFloat(parts[0], 64)
	l5, err2 := strconv.ParseFloat(parts[1], 64)
	l15, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, fmt.Errorf("parse loadavg values")
	}
	return l1, l5, l15, nil
}

func parseMemInfoLinux() (int64, int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var totalKB, availableKB int64
	found := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				totalKB, _ = strconv.ParseInt(fields[1], 10, 64)
				found++
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				availableKB, _ = strconv.ParseInt(fields[1], 10, 64)
				found++
			}
		}
		if found == 2 {
			break
		}
	}
	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found")
	}
	totalMB := totalKB / 1024
	usedMB := (totalKB - availableKB) / 1024
	return totalMB, usedMB, nil
}

func parseDiskUsageLinux() (int64, int64, error) {
	out, err := exec.Command("df", "-k", "/").Output()
	if err != nil {
		return 0, 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, 0, fmt.Errorf("unexpected df output")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, 0, fmt.Errorf("unexpected df fields")
	}
	totalKB, err1 := strconv.ParseInt(fields[1], 10, 64)
	usedKB, err2 := strconv.ParseInt(fields[2], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("parse df values")
	}
	return totalKB / 1024, usedKB / 1024, nil
}
