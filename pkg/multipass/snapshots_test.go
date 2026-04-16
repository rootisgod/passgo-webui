package multipass

import (
	"testing"
)

const snapshotListJSON = `{
  "errors": [],
  "info": {
    "vm-a": {
      "snap1": {"parent": "", "comment": "first"},
      "snap2": {"parent": "snap1", "comment": "second"}
    },
    "vm-b": {
      "snapX": {"parent": "", "comment": ""}
    }
  }
}`

func TestListSnapshots_SingleVM(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"list --snapshots --format json": snapshotListJSON,
	})
	c := NewClientWithRunner(discardLogger(), runner)
	snaps, err := c.ListSnapshots("vm-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}
	// Find each snapshot and assert fields — order isn't guaranteed from map iteration.
	names := map[string]SnapshotInfo{}
	for _, s := range snaps {
		names[s.Name] = s
	}
	if s, ok := names["snap1"]; !ok || s.Comment != "first" || s.Instance != "vm-a" {
		t.Errorf("snap1 wrong: %+v", s)
	}
	if s, ok := names["snap2"]; !ok || s.Parent != "snap1" {
		t.Errorf("snap2 wrong: %+v", s)
	}
}

func TestListSnapshots_NoSnapshotsForVM(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"list --snapshots --format json": snapshotListJSON,
	})
	c := NewClientWithRunner(discardLogger(), runner)
	snaps, err := c.ListSnapshots("does-not-exist")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("want 0, got %d", len(snaps))
	}
}

func TestListSnapshots_RejectsInvalidVMName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if _, err := c.ListSnapshots("--all"); err == nil {
		t.Error("expected validation error")
	}
}

func TestCreateSnapshot_WithComment(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"snapshot --name snap1 --comment hello vm-a": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.CreateSnapshot("vm-a", "snap1", "hello"); err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestCreateSnapshot_NoComment(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"snapshot --name snap1 vm-a": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.CreateSnapshot("vm-a", "snap1", ""); err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestCreateSnapshot_RejectsInvalidVM(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if err := c.CreateSnapshot("--all", "s", ""); err == nil {
		t.Error("expected validation error on vm")
	}
}

func TestCreateSnapshot_RejectsInvalidSnapshotName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if err := c.CreateSnapshot("vm", "--evil", ""); err == nil {
		t.Error("expected validation error on snapshot name")
	}
}

func TestRestoreSnapshot_ArgConstruction(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"restore --destructive vm-a.snap1": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.RestoreSnapshot("vm-a", "snap1"); err != nil {
		t.Fatalf("restore: %v", err)
	}
}

func TestRestoreSnapshot_RejectsInvalidSnapshotName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if err := c.RestoreSnapshot("vm", "--evil"); err == nil {
		t.Error("expected validation error")
	}
}

func TestDeleteSnapshot_ArgConstruction(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"delete --purge vm-a.snap1": "ok",
	})
	c := NewClientWithRunner(discardLogger(), runner)
	if err := c.DeleteSnapshot("vm-a", "snap1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
