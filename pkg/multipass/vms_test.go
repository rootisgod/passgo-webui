package multipass

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// --- parseInfoJSON ---

const infoJSONSingleVM = `{
  "errors": [],
  "info": {
    "test-vm": {
      "state": "Running",
      "image_hash": "abc123",
      "image_release": "24.04 LTS",
      "release": "Ubuntu 24.04.1 LTS",
      "cpu_count": "2",
      "load": [0.5, 0.3, 0.1],
      "disks": {"sda1": {"used": "1073741824", "total": "5368709120"}},
      "memory": {"used": 536870912, "total": 2147483648},
      "mounts": {
        "/home/ubuntu/data": {"source_path": "/host/data", "uid_mappings": ["1000:1000"], "gid_mappings": ["1000:1000"]}
      },
      "ipv4": ["10.0.0.5"],
      "snapshots": {"snap1": {"parent": "", "comment": ""}}
    }
  }
}`

const infoJSONMultiVM = `{
  "errors": [],
  "info": {
    "vm-b": {"state": "Stopped", "cpu_count": "1", "memory": {"used": 0, "total": 1073741824}, "disks": {}, "mounts": {}, "ipv4": [], "snapshots": {}},
    "vm-a": {"state": "Running", "cpu_count": "4", "memory": {"used": 0, "total": 0}, "disks": {}, "mounts": {}, "ipv4": [], "snapshots": {}}
  }
}`

func TestParseInfoJSON_SingleVM(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), nil)
	vms, err := c.parseInfoJSON(infoJSONSingleVM)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("want 1 VM, got %d", len(vms))
	}
	vm := vms[0]
	if vm.Name != "test-vm" || vm.State != "Running" || vm.CPUs != "2" {
		t.Errorf("unexpected VM: %+v", vm)
	}
	if vm.Load != "0.50 0.30 0.10" {
		t.Errorf("load: got %q", vm.Load)
	}
	if vm.MemoryUsageRaw != 536870912 || vm.MemoryTotalRaw != 2147483648 {
		t.Errorf("memory raw: %d / %d", vm.MemoryUsageRaw, vm.MemoryTotalRaw)
	}
	if vm.MemoryUsage != "512.0 MiB" || vm.MemoryTotal != "2.0 GiB" {
		t.Errorf("memory fmt: %q / %q", vm.MemoryUsage, vm.MemoryTotal)
	}
	if vm.Snapshots != 1 {
		t.Errorf("snapshots: want 1, got %d", vm.Snapshots)
	}
	if len(vm.Mounts) != 1 || vm.Mounts[0].TargetPath != "/home/ubuntu/data" {
		t.Errorf("mounts: %+v", vm.Mounts)
	}
}

func TestParseInfoJSON_MultiVMSortedByName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), nil)
	vms, err := c.parseInfoJSON(infoJSONMultiVM)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vms) != 2 {
		t.Fatalf("want 2 VMs, got %d", len(vms))
	}
	// Must be sorted alphabetically regardless of input map order.
	if vms[0].Name != "vm-a" || vms[1].Name != "vm-b" {
		t.Errorf("not sorted: %s, %s", vms[0].Name, vms[1].Name)
	}
}

func TestParseInfoJSON_Malformed(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), nil)
	if _, err := c.parseInfoJSON("not json"); err == nil {
		t.Error("want error for malformed JSON")
	}
}

// --- LaunchVM arg construction ---

func TestLaunchVM_ArgConstruction(t *testing.T) {
	runner, calls := fakeRunner(t, map[string]string{
		"launch --name my-vm --cpus 2 --memory 2048M --disk 10G 24.04": "launched",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	name, err := c.LaunchVM("my-vm", "24.04", 2, 2048, 10, "", "")
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if name != "my-vm" {
		t.Errorf("name: got %q", name)
	}
	if len(*calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(*calls))
	}
}

func TestLaunchVM_BridgedNetwork(t *testing.T) {
	runner, calls := fakeRunner(t, map[string]string{
		"launch --name v1 --cpus 2 --memory 1024M --disk 8G --bridged 24.04": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if _, err := c.LaunchVM("v1", "24.04", 2, 1024, 8, "", "bridged"); err != nil {
		t.Fatalf("launch: %v", err)
	}
	argv := (*calls)[0]
	var hasBridged bool
	for _, a := range argv {
		if a == "--bridged" {
			hasBridged = true
		}
	}
	if !hasBridged {
		t.Errorf("expected --bridged in argv: %v", argv)
	}
}

func TestLaunchVM_NamedNetwork(t *testing.T) {
	runner, calls := fakeRunner(t, map[string]string{
		"launch --name v1 --cpus 2 --memory 1024M --disk 8G --network en0 24.04": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if _, err := c.LaunchVM("v1", "24.04", 2, 1024, 8, "", "en0"); err != nil {
		t.Fatalf("launch: %v", err)
	}
	argv := (*calls)[0]
	for i, a := range argv {
		if a == "--network" && i+1 < len(argv) && argv[i+1] == "en0" {
			return
		}
	}
	t.Errorf("expected --network en0 in argv: %v", argv)
}

func TestLaunchVM_ClampsBelowMinimums(t *testing.T) {
	// CPUs=0 should clamp to DefaultCPUCores; memory below MinRAMMB clamps; disk below MinDiskGB clamps.
	var capturedArgs []string
	runner := func(args ...string) (string, error) {
		capturedArgs = args
		return "ok", nil
	}
	c := NewClientWithRunner(discardLogger(), runner)
	if _, err := c.LaunchVM("vm", "", 0, 10, 0, "", ""); err != nil {
		t.Fatalf("launch: %v", err)
	}
	joined := strings.Join(capturedArgs, " ")
	// Defaults should have been applied.
	if !strings.Contains(joined, "--cpus 2") {
		t.Errorf("expected default cpus, got: %s", joined)
	}
	if !strings.Contains(joined, "--memory 1024M") {
		t.Errorf("expected default memory, got: %s", joined)
	}
	if !strings.Contains(joined, "--disk 8G") {
		t.Errorf("expected default disk, got: %s", joined)
	}
	// Release defaults to 24.04.
	if !strings.HasSuffix(joined, " 24.04") {
		t.Errorf("expected default release at end, got: %s", joined)
	}
}

func TestLaunchVM_RejectsFlagInjection(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	_, err := c.LaunchVM("--all", "24.04", 2, 1024, 8, "", "")
	if err == nil {
		t.Fatal("expected validation error for --all name")
	}
	if !strings.Contains(err.Error(), "invalid VM name") {
		t.Errorf("want invalid VM name error, got: %v", err)
	}
}

// --- CloneVM ---

func TestCloneVM_WithDestName(t *testing.T) {
	runner, calls := fakeRunner(t, map[string]string{
		"clone src --name dst": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if _, err := c.CloneVM("src", "dst"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("want 1 call, got %d", len(*calls))
	}
}

func TestCloneVM_NoDestName(t *testing.T) {
	runner, calls := fakeRunner(t, map[string]string{
		"clone src": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if _, err := c.CloneVM("src", ""); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("want 1 call, got %d", len(*calls))
	}
}

func TestCloneVM_RejectsInvalidSource(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if _, err := c.CloneVM("--all", "dst"); err == nil {
		t.Error("expected validation error on source")
	}
}

func TestCloneVM_RejectsInvalidDest(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if _, err := c.CloneVM("src", "--all"); err == nil {
		t.Error("expected validation error on dest")
	}
}

// --- Start/Stop/Delete/etc simple arg tests ---

func TestStartVM(t *testing.T) {
	runner, calls := fakeRunner(t, map[string]string{"start vm": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.StartVM("vm"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("calls: %d", len(*calls))
	}
}

func TestDeleteVM_WithPurge(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{"delete vm --purge": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.DeleteVM("vm", true); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestDeleteVM_NoPurge(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{"delete vm": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.DeleteVM("vm", false); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestDeleteVM_RejectsFlagInjection(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if err := c.DeleteVM("--all", true); err == nil {
		t.Error("expected validation error")
	}
}

func TestRecoverVM_RejectsInvalidName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if err := c.RecoverVM("-foo"); err == nil {
		t.Error("expected validation error")
	}
}

// --- StartAll / StopAll filtering ---

const mixedStateInfo = `{"errors":[],"info":{
  "stopped-a": {"state":"Stopped","cpu_count":"1","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":[],"snapshots":{}},
  "running-b": {"state":"Running","cpu_count":"1","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":[],"snapshots":{}},
  "stopped-c": {"state":"Stopped","cpu_count":"1","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":[],"snapshots":{}}
}}`

func TestStartAll_OnlyStopped(t *testing.T) {
	// StartAll calls ListVMs then StartVM for each Stopped VM.
	var startCalls []string
	runner := func(args ...string) (string, error) {
		if args[0] == "info" && args[1] == "--all" {
			return mixedStateInfo, nil
		}
		if args[0] == "start" {
			startCalls = append(startCalls, args[1])
			return "", nil
		}
		return "", errors.New("unexpected: " + strings.Join(args, " "))
	}
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.StartAll(); err != nil {
		t.Fatalf("start-all: %v", err)
	}
	// Names come out of a Go map — assert as a set, not order.
	got := map[string]bool{}
	for _, n := range startCalls {
		got[n] = true
	}
	want := map[string]bool{"stopped-a": true, "stopped-c": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("start calls: got %v, want %v", got, want)
	}
}

func TestStopAll_OnlyRunning(t *testing.T) {
	var stopCalls []string
	runner := func(args ...string) (string, error) {
		if args[0] == "info" && args[1] == "--all" {
			return mixedStateInfo, nil
		}
		if args[0] == "stop" {
			stopCalls = append(stopCalls, args[1])
			return "", nil
		}
		return "", errors.New("unexpected: " + strings.Join(args, " "))
	}
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.StopAll(); err != nil {
		t.Fatalf("stop-all: %v", err)
	}
	if len(stopCalls) != 1 || stopCalls[0] != "running-b" {
		t.Errorf("stop calls: %v", stopCalls)
	}
}

// --- Memory / disk parsing ---

func TestParseMemoryToMB(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"1073741824", 1024}, // bytes
		{"1024M", 1024},
		{"2048m", 2048},
		{"1G", 1024},
		{"1g", 1024},
		{"2.5G", 2560},
		{"1.0GiB", 1024},
		{"512MiB", 512},
		{"1024KiB", 1},
		{"1T", 1024 * 1024},
		{"bogus", 0},
	}
	for _, tc := range cases {
		if got := parseMemoryToMB(tc.in); got != tc.want {
			t.Errorf("parseMemoryToMB(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseDiskToGB(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"5368709120", 5}, // bytes → 5 GB
		{"8G", 8},
		{"8g", 8},
		{"8.5G", 8},
		{"8.0GiB", 8},
		{"1024M", 1},
		{"1T", 1024},
		{"bogus", 0},
	}
	for _, tc := range cases {
		if got := parseDiskToGB(tc.in); got != tc.want {
			t.Errorf("parseDiskToGB(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// --- GetVMConfig with malformed CLI output ---

func TestGetVMConfig_MalformedValues(t *testing.T) {
	runner := func(args ...string) (string, error) {
		// multipass get local.<name>.<key>
		if len(args) == 2 && args[0] == "get" {
			switch args[1] {
			case "local.vm.cpus":
				return "not-a-number", nil
			case "local.vm.memory":
				return "garbage", nil
			case "local.vm.disk":
				return "", nil
			}
		}
		return "", errors.New("unexpected: " + strings.Join(args, " "))
	}
	c := NewClientWithRunner(discardLogger(), runner)
	cfg, err := c.GetVMConfig("vm")
	if err != nil {
		t.Fatalf("GetVMConfig: %v", err)
	}
	// Malformed values yield zero, not an error (logged as warnings).
	if cfg.CPUs != 0 || cfg.MemoryMB != 0 || cfg.DiskGB != 0 {
		t.Errorf("expected zero values, got %+v", cfg)
	}
}

// --- Exec rejects invalid names ---

func TestExecInVM_RejectsFlagInjection(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if _, err := c.ExecInVM("--all", []string{"ls"}); err == nil {
		t.Error("expected validation error")
	}
}

// --- GetVMInfo + ListVMs paths ---

func TestGetVMInfo_Found(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"info test-vm --format json": infoJSONSingleVM,
	})
	c := NewClientWithRunner(discardLogger(), runner)
	vm, err := c.GetVMInfo("test-vm")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if vm.Name != "test-vm" || vm.State != "Running" {
		t.Errorf("got %+v", vm)
	}
}

func TestGetVMInfo_RejectsInvalidName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if _, err := c.GetVMInfo("--all"); err == nil {
		t.Error("expected validation error")
	}
}

func TestListVMs_FallsBackToListOnInfoFailure(t *testing.T) {
	// info --all fails, so the code falls back to `list --format json`.
	const listJSON = `{"list":[{"name":"a","state":"Stopped","ipv4":[],"release":"Ubuntu 24.04"}]}`
	runner := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if joined == "info --all --format json" {
			return "", errNoVMs // simulate multipass "no VMs" failure
		}
		if joined == "list --format json" {
			return listJSON, nil
		}
		t.Errorf("unexpected call: %v", args)
		return "", nil
	}
	c := NewClientWithRunner(discardLogger(), runner)
	vms, err := c.ListVMs()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(vms) != 1 || vms[0].Name != "a" {
		t.Errorf("got %+v", vms)
	}
}

// errNoVMs is a shared sentinel for tests that need to simulate a multipass failure.
var errNoVMs = stubError("no VMs")

type stubError string

func (e stubError) Error() string { return string(e) }

// --- Suspend / Purge / Set* / Resolve simple cases ---

func TestSuspendVM(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{"suspend vm": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.SuspendVM("vm"); err != nil {
		t.Fatalf("suspend: %v", err)
	}
}

func TestSuspendVM_RejectsInvalidName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if err := c.SuspendVM("--all"); err == nil {
		t.Error("expected validation error")
	}
}

func TestPurgeDeleted(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{"purge": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.PurgeDeleted(); err != nil {
		t.Fatalf("purge: %v", err)
	}
}

func TestSetVMCPUs(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{"set local.vm.cpus=4": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.SetVMCPUs("vm", 4); err != nil {
		t.Fatalf("set cpus: %v", err)
	}
}

func TestSetVMMemory(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{"set local.vm.memory=2048M": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.SetVMMemory("vm", 2048); err != nil {
		t.Fatalf("set memory: %v", err)
	}
}

func TestSetVMDisk(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{"set local.vm.disk=20G": "ok"})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.SetVMDisk("vm", 20); err != nil {
		t.Fatalf("set disk: %v", err)
	}
}

func TestSetVMCPUs_RejectsInvalidName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if err := c.SetVMCPUs("--all", 4); err == nil {
		t.Error("expected validation error")
	}
}

func TestResolveLaunchName_EmptyGetsRandom(t *testing.T) {
	name := ResolveLaunchName("")
	if !strings.HasPrefix(name, VMNamePrefix) {
		t.Errorf("expected random name with prefix %q, got %q", VMNamePrefix, name)
	}
}

func TestResolveLaunchName_Passthrough(t *testing.T) {
	if got := ResolveLaunchName("my-vm"); got != "my-vm" {
		t.Errorf("passthrough: got %q", got)
	}
}

// --- formatBytes ---

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1024, "1024 B"},
		{1024 * 1024, "1.0 MiB"},
		{5 * 1024 * 1024, "5.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
		{2 * 1024 * 1024 * 1024, "2.0 GiB"},
	}
	for _, tc := range cases {
		if got := formatBytes(tc.in); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
