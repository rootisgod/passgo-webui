package multipass

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ListVMs returns full details for all VMs using JSON output.
func (c *Client) ListVMs() ([]VMInfo, error) {
	output, err := c.run("info", "--all", "--format", "json")
	if err != nil {
		// If no VMs exist, multipass info --all fails. Try list first.
		listOutput, listErr := c.run("list", "--format", "json")
		if listErr != nil {
			return nil, fmt.Errorf("list VMs: %w", err)
		}
		var listResp listJSONResponse
		if err := json.Unmarshal([]byte(listOutput), &listResp); err != nil {
			return nil, fmt.Errorf("parse VM list: %w", err)
		}
		if len(listResp.List) == 0 {
			return []VMInfo{}, nil
		}
		// If there are VMs but info --all failed, return basic info from list
		var vms []VMInfo
		for _, v := range listResp.List {
			vms = append(vms, VMInfo{
				Name:    v.Name,
				State:   v.State,
				IPv4:    v.IPv4,
				Release: v.Release,
			})
		}
		sort.Slice(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name })
		return vms, nil
	}

	return c.parseInfoJSON(output)
}

// GetVMInfo returns details for a single VM.
func (c *Client) GetVMInfo(name string) (VMInfo, error) {
	output, err := c.run("info", name, "--format", "json")
	if err != nil {
		return VMInfo{}, fmt.Errorf("get VM info: %w", err)
	}
	vms, err := c.parseInfoJSON(output)
	if err != nil {
		return VMInfo{}, err
	}
	for _, vm := range vms {
		if vm.Name == name {
			return vm, nil
		}
	}
	return VMInfo{}, fmt.Errorf("VM %q not found in info response", name)
}

func (c *Client) parseInfoJSON(output string) ([]VMInfo, error) {
	var resp infoJSONResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse VM info JSON: %w", err)
	}

	var vms []VMInfo
	for name, detail := range resp.Info {
		vm := VMInfo{
			Name:      name,
			State:     detail.State,
			IPv4:      detail.IPv4,
			Release:   detail.Release,
			ImageHash: detail.ImageHash,
			CPUs:      detail.CPUs,
			Snapshots: len(detail.Snapshots),
		}

		if len(detail.Load) == 3 {
			vm.Load = fmt.Sprintf("%.2f %.2f %.2f", detail.Load[0], detail.Load[1], detail.Load[2])
		}

		for _, disk := range detail.Disks {
			if used, err := strconv.ParseInt(disk.Used, 10, 64); err == nil {
				vm.DiskUsageRaw = used
				vm.DiskUsage = formatBytes(used)
			}
			if total, err := strconv.ParseInt(disk.Total, 10, 64); err == nil {
				vm.DiskTotalRaw = total
				vm.DiskTotal = formatBytes(total)
			}
			break
		}

		vm.MemoryUsageRaw = detail.Memory.Used
		vm.MemoryTotalRaw = detail.Memory.Total
		vm.MemoryUsage = formatBytes(detail.Memory.Used)
		vm.MemoryTotal = formatBytes(detail.Memory.Total)

		for target, mount := range detail.Mounts {
			vm.Mounts = append(vm.Mounts, MountInfo{
				SourcePath: mount.SourcePath,
				TargetPath: target,
				UIDMaps:    mount.UIDMappings,
				GIDMaps:    mount.GIDMappings,
			})
		}
		sort.Slice(vm.Mounts, func(i, j int) bool {
			return vm.Mounts[i].TargetPath < vm.Mounts[j].TargetPath
		})

		vms = append(vms, vm)
	}

	sort.Slice(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name })
	return vms, nil
}

func formatBytes(b int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(mb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// ResolveLaunchName returns the name that will be used for a VM, generating one if empty.
func ResolveLaunchName(name string) string {
	if name == "" {
		return RandomVMName()
	}
	return name
}

// LaunchVM creates a new VM with the given options.
func (c *Client) LaunchVM(name, release string, cpus, memoryMB, diskGB int, cloudInitFile, networkName string) (string, error) {
	if name == "" {
		name = RandomVMName()
	}
	if release == "" {
		release = DefaultUbuntuRelease
	}
	if cpus < MinCPUCores {
		cpus = DefaultCPUCores
	}
	if memoryMB < MinRAMMB {
		memoryMB = DefaultRAMMB
	}
	if diskGB < MinDiskGB {
		diskGB = DefaultDiskGB
	}

	args := []string{
		"launch",
		"--name", name,
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%dM", memoryMB),
		"--disk", fmt.Sprintf("%dG", diskGB),
	}

	if cloudInitFile != "" {
		args = append(args, "--cloud-init", cloudInitFile)
	}

	if networkName == "bridged" {
		args = append(args, "--bridged")
	} else if networkName != "" {
		args = append(args, "--network", networkName)
	}

	args = append(args, release)
	_, err := c.run(args...)
	return name, err
}

// CloneVM creates an independent copy of a stopped VM.
// If destName is empty, multipass auto-generates a name like <source>-clone1.
func (c *Client) CloneVM(source, destName string) (string, error) {
	args := []string{"clone", source}
	if destName != "" {
		args = append(args, "--name", destName)
	}
	_, err := c.run(args...)
	return destName, err
}

// StartVM starts a stopped VM.
func (c *Client) StartVM(name string) error {
	_, err := c.run("start", name)
	return err
}

// StopVM stops a running VM.
func (c *Client) StopVM(name string) error {
	_, err := c.run("stop", name)
	return err
}

// SuspendVM suspends a running VM.
func (c *Client) SuspendVM(name string) error {
	_, err := c.run("suspend", name)
	return err
}

// DeleteVM deletes a VM. If purge is true, also purges it.
func (c *Client) DeleteVM(name string, purge bool) error {
	args := []string{"delete", name}
	if purge {
		args = append(args, "--purge")
	}
	_, err := c.run(args...)
	return err
}

// RecoverVM recovers a deleted VM.
func (c *Client) RecoverVM(name string) error {
	_, err := c.run("recover", name)
	return err
}

// PurgeDeleted purges all deleted VMs.
func (c *Client) PurgeDeleted() error {
	_, err := c.run("purge")
	return err
}

// StartAll starts all stopped VMs.
func (c *Client) StartAll() error {
	vms, err := c.ListVMs()
	if err != nil {
		return err
	}
	var errs []string
	for _, vm := range vms {
		if vm.State == "Stopped" {
			if err := c.StartVM(vm.Name); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", vm.Name, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("start-all errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// StopAll stops all running VMs.
func (c *Client) StopAll() error {
	vms, err := c.ListVMs()
	if err != nil {
		return err
	}
	var errs []string
	for _, vm := range vms {
		if vm.State == "Running" {
			if err := c.StopVM(vm.Name); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", vm.Name, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("stop-all errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ExecInVM runs a command inside a VM and returns stdout.
func (c *Client) ExecInVM(vmName string, command []string) (string, error) {
	args := append([]string{"exec", vmName, "--"}, command...)
	return c.run(args...)
}

// ExecInVMWithContext runs a command inside a VM with a context for timeout/cancellation.
func (c *Client) ExecInVMWithContext(ctx context.Context, vmName string, command []string) (string, error) {
	args := append([]string{"exec", vmName, "--"}, command...)
	return c.runWithContext(ctx, args...)
}

// ExecInVMStreaming runs a command inside a VM, streaming stdout lines to a callback.
func (c *Client) ExecInVMStreaming(ctx context.Context, vmName string, command []string, onLine func(string)) (string, error) {
	args := append([]string{"exec", vmName, "--"}, command...)
	return c.runStreamingContext(ctx, onLine, args...)
}

// VMConfig holds the configured (not runtime) resource specs for a VM.
type VMConfig struct {
	CPUs     int   `json:"cpus"`
	MemoryMB int64 `json:"memory_mb"`
	DiskGB   int64 `json:"disk_gb"`
}

// GetVMConfig reads the configured CPU, memory, and disk for a VM via multipass get.
// Unlike info, these values are available even when the VM is stopped.
func (c *Client) GetVMConfig(name string) (VMConfig, error) {
	var cfg VMConfig

	cpuStr, err := c.run("get", fmt.Sprintf("local.%s.cpus", name))
	if err != nil {
		return cfg, fmt.Errorf("get cpus: %w", err)
	}
	cfg.CPUs, _ = strconv.Atoi(strings.TrimSpace(cpuStr))

	memStr, err := c.run("get", fmt.Sprintf("local.%s.memory", name))
	if err != nil {
		return cfg, fmt.Errorf("get memory: %w", err)
	}
	cfg.MemoryMB = parseMemoryToMB(strings.TrimSpace(memStr))

	diskStr, err := c.run("get", fmt.Sprintf("local.%s.disk", name))
	if err != nil {
		return cfg, fmt.Errorf("get disk: %w", err)
	}
	cfg.DiskGB = parseDiskToGB(strings.TrimSpace(diskStr))

	return cfg, nil
}

// stripUnitSuffix strips unit suffixes like "GiB", "MiB", "G", "M" and returns
// the numeric part and the unit letter (G, M, K, T) or 'B' for plain bytes.
func stripUnitSuffix(s string) (string, byte) {
	// Handle GiB/MiB/KiB/TiB suffixes (e.g. "1.0GiB")
	for _, suffix := range []string{"GiB", "MiB", "KiB", "TiB"} {
		if strings.HasSuffix(s, suffix) {
			return s[:len(s)-len(suffix)], suffix[0]
		}
	}
	// Handle GB/MB/KB/TB suffixes
	for _, suffix := range []string{"GB", "MB", "KB", "TB"} {
		if strings.HasSuffix(s, suffix) {
			return s[:len(s)-len(suffix)], suffix[0]
		}
	}
	// Handle single-letter suffixes (G, M, K, T)
	if len(s) > 0 {
		last := s[len(s)-1]
		switch last {
		case 'G', 'g', 'M', 'm', 'K', 'k', 'T', 't':
			return s[:len(s)-1], last
		}
	}
	return s, 'B' // plain number = bytes
}

// parseMemoryToMB parses a memory string like "1073741824", "1024M", "1.0GiB" to MB.
func parseMemoryToMB(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	num, unit := stripUnitSuffix(s)
	switch unit {
	case 'G', 'g':
		v, _ := strconv.ParseFloat(num, 64)
		return int64(v * 1024)
	case 'M', 'm':
		v, _ := strconv.ParseFloat(num, 64)
		return int64(v)
	case 'K', 'k':
		v, _ := strconv.ParseFloat(num, 64)
		return int64(v / 1024)
	case 'T', 't':
		v, _ := strconv.ParseFloat(num, 64)
		return int64(v * 1024 * 1024)
	default:
		// Plain number = bytes
		v, _ := strconv.ParseInt(num, 10, 64)
		return v / (1024 * 1024)
	}
}

// parseDiskToGB parses a disk string like "5368709120", "8G", "8.0GiB" to GB.
func parseDiskToGB(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	num, unit := stripUnitSuffix(s)
	switch unit {
	case 'G', 'g':
		v, _ := strconv.ParseFloat(num, 64)
		return int64(v)
	case 'M', 'm':
		v, _ := strconv.ParseFloat(num, 64)
		return int64(v / 1024)
	case 'T', 't':
		v, _ := strconv.ParseFloat(num, 64)
		return int64(v * 1024)
	default:
		// Plain number = bytes
		v, _ := strconv.ParseInt(num, 10, 64)
		return v / (1024 * 1024 * 1024)
	}
}

// SetVMCPUs changes the number of CPUs allocated to a VM (requires VM to be stopped).
func (c *Client) SetVMCPUs(name string, cpus int) error {
	_, err := c.run("set", fmt.Sprintf("local.%s.cpus=%d", name, cpus))
	return err
}

// SetVMMemory changes the memory allocated to a VM in MB (requires VM to be stopped).
func (c *Client) SetVMMemory(name string, memoryMB int) error {
	_, err := c.run("set", fmt.Sprintf("local.%s.memory=%dM", name, memoryMB))
	return err
}

// SetVMDisk changes the disk size of a VM in GB (can only increase, works while running).
func (c *Client) SetVMDisk(name string, diskGB int) error {
	_, err := c.run("set", fmt.Sprintf("local.%s.disk=%dG", name, diskGB))
	return err
}

// GetRawInfo returns the raw text output of multipass info for a VM.
func (c *Client) GetRawInfo(name string) (string, error) {
	return c.run("info", name)
}

// CloudInitStatus represents the status of cloud-init inside a VM.
type CloudInitStatus struct {
	Status string `json:"status"` // "running", "done", "error", "disabled", "not started"
	Detail string `json:"detail"`
	Errors []string `json:"errors,omitempty"`
	Output string `json:"output,omitempty"` // last N lines of cloud-init output log
}

// GetCloudInitStatus checks the cloud-init status inside a running VM.
// Note: cloud-init status exits 0=running/done, 1=fatal error, 2=recoverable error.
// All are valid states with useful stdout, so we use "sh -c '... ; exit 0'" to always
// capture the output regardless of exit code.
func (c *Client) GetCloudInitStatus(vmName string) (CloudInitStatus, error) {
	result := CloudInitStatus{Status: "pending"}

	// Try JSON format first (cloud-init 22.1+, available on Ubuntu 22.04+)
	// Wrap in sh to capture stdout even on non-zero exit (exit 2 = recoverable error)
	output, err := c.run("exec", vmName, "--", "sh", "-c", "cloud-init status --format json 2>/dev/null; exit 0")
	if err == nil && strings.TrimSpace(output) != "" {
		var parsed struct {
			Status         string   `json:"status"`
			Detail         string   `json:"detail"`
			Errors         []string `json:"errors"`
			ExtendedStatus string   `json:"extended_status"`
		}
		if jsonErr := json.Unmarshal([]byte(output), &parsed); jsonErr == nil {
			result.Status = parsed.Status
			if parsed.ExtendedStatus != "" {
				result.Status = parsed.ExtendedStatus
			}
			result.Detail = parsed.Detail
			result.Errors = parsed.Errors
		}
	} else if err != nil {
		// exec itself failed (VM not ready for SSH yet) -- return pending, not an error
		result.Detail = "Waiting for VM to be ready..."
		return result, nil
	}

	// Get last 50 lines of output log (best effort)
	logOutput, _ := c.run("exec", vmName, "--", "sh", "-c", "tail -n 50 /var/log/cloud-init-output.log 2>/dev/null; exit 0")
	if logOutput != "" {
		result.Output = logOutput
	}

	return result, nil
}
