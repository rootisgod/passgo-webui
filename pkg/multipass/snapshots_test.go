package multipass

import (
	"strings"
	"testing"
)

const snapshotInfoJSON = `{
  "errors": [],
  "info": {
    "vm-a": {
      "snapshots": {
        "snap1": {"parent": "", "comment": "first", "created": "Thu 16 Apr 20:52:16 2026 BST", "children": ["snap2"]},
        "snap2": {"parent": "snap1", "comment": "second", "created": "Thu 16 Apr 20:52:24 2026 BST", "children": []}
      }
    }
  }
}`

func TestListSnapshots_ParsesRichFields(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"info vm-a --snapshots --format json": snapshotInfoJSON,
	})
	c := NewClientWithRunner(discardLogger(), runner)
	snaps, err := c.ListSnapshots("vm-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}
	byName := map[string]SnapshotInfo{}
	for _, s := range snaps {
		byName[s.Name] = s
	}

	snap1 := byName["snap1"]
	if snap1.Comment != "first" || snap1.Instance != "vm-a" {
		t.Errorf("snap1 fields wrong: %+v", snap1)
	}
	// Multipass "Thu 16 Apr 20:52:16 2026 BST" normalises to UTC RFC3339.
	// BST = UTC+1 → 20:52:16 BST = 19:52:16 UTC.
	if !strings.HasPrefix(snap1.Created, "2026-04-16T19:52:16") {
		t.Errorf("snap1.Created = %q, want prefix 2026-04-16T19:52:16 (UTC)", snap1.Created)
	}
	if len(snap1.Children) != 1 || snap1.Children[0] != "snap2" {
		t.Errorf("snap1.Children = %v", snap1.Children)
	}

	if snap2 := byName["snap2"]; snap2.Parent != "snap1" {
		t.Errorf("snap2.Parent = %q", snap2.Parent)
	}
}

func TestListSnapshots_NoSnapshotsForVM(t *testing.T) {
	// multipass info for a VM with no snapshots returns info.<vm>.snapshots={}.
	runner, _ := fakeRunner(t, map[string]string{
		"info vm-a --snapshots --format json": `{"errors":[],"info":{"vm-a":{"snapshots":{}}}}`,
	})
	c := NewClientWithRunner(discardLogger(), runner)
	snaps, err := c.ListSnapshots("vm-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("want 0 snapshots, got %d", len(snaps))
	}
}

func TestListSnapshots_RejectsInvalidVMName(t *testing.T) {
	c := NewClientWithRunner(discardLogger(), noCallRunner(t))
	if _, err := c.ListSnapshots("--all"); err == nil {
		t.Error("expected validation error")
	}
}

// TestListSnapshots_RealCapture runs against an actual
// `multipass info <vm> --snapshots --format json` output to catch format drift.
func TestListSnapshots_RealCapture(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"info ansible --snapshots --format json": loadFixture(t, "info_snapshots.json"),
	})
	c := NewClientWithRunner(discardLogger(), runner)
	snaps, err := c.ListSnapshots("ansible")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(snaps) != 4 {
		t.Fatalf("want 4 snapshots, got %d", len(snaps))
	}
	byName := map[string]SnapshotInfo{}
	for _, s := range snaps {
		byName[s.Name] = s
	}
	// Parent chain checks.
	if byName["test-1"].Parent != "" {
		t.Errorf("test-1 parent: %q", byName["test-1"].Parent)
	}
	if byName["another-snaphot"].Parent != "other-snapshot" {
		t.Errorf("another-snaphot parent: %q", byName["another-snaphot"].Parent)
	}
	// Children are captured.
	if len(byName["test-1"].Children) != 2 {
		t.Errorf("test-1 children: %v", byName["test-1"].Children)
	}
	// Created timestamps parsed to UTC. extra-snapshot was taken last.
	// BST = UTC+1, so 20:53:27 BST = 19:53:27 UTC.
	if !strings.HasPrefix(byName["extra-snapshot"].Created, "2026-04-16T19:53:27") {
		t.Errorf("extra-snapshot.Created = %q", byName["extra-snapshot"].Created)
	}
	// Newest-by-Created heuristic: extra-snapshot should come out newest.
	var newest SnapshotInfo
	for _, s := range snaps {
		if s.Created > newest.Created {
			newest = s
		}
	}
	if newest.Name != "extra-snapshot" {
		t.Errorf("newest by Created = %q, want extra-snapshot", newest.Name)
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

func TestParseMultipassCreated(t *testing.T) {
	cases := []struct {
		in   string
		want string // expected prefix — full value may include timezone suffix
	}{
		{"", ""},
		// Both observed multipass layouts normalise to the same UTC instant.
		{"Thu Apr 16 20:52:16 2026 BST", "2026-04-16T19:52:16"}, // ANSIC-like, month-before-day
		{"Thu 16 Apr 20:52:16 2026 BST", "2026-04-16T19:52:16"}, // day-before-month
		{"2026-04-16T19:52:16Z", "2026-04-16T19:52:16"},
		{"garbage string no one expects", "garbage string no one expects"}, // passthrough
	}
	for _, tc := range cases {
		got := parseMultipassCreated(tc.in)
		if !strings.HasPrefix(got, tc.want) {
			t.Errorf("parseMultipassCreated(%q) = %q, want prefix %q", tc.in, got, tc.want)
		}
	}
}
