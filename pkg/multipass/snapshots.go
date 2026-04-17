package multipass

import (
	"encoding/json"
	"fmt"
	"time"
)

// snapshotInfoJSONResponse is the response from multipass info --snapshots --format json.
// Richer than list --snapshots: includes created timestamps and children arrays,
// which let the UI default "current" to newest-by-created and surface context.
type snapshotInfoJSONResponse struct {
	Errors []string                               `json:"errors"`
	Info   map[string]snapshotInfoJSONVMSnapshots `json:"info"`
}

type snapshotInfoJSONVMSnapshots struct {
	Snapshots map[string]snapshotInfoJSONEntry `json:"snapshots"`
}

type snapshotInfoJSONEntry struct {
	Parent   string   `json:"parent"`
	Comment  string   `json:"comment"`
	Created  string   `json:"created"`
	Children []string `json:"children"`
}

// multipassCreatedLayouts lists the formats multipass has been observed to use
// for the "created" field. Multipass varies by locale/version — observed so far:
//   - "Thu Apr 16 20:52:52 2026 BST"   (ANSIC-like, month before day)
//   - "Thu 16 Apr 20:52:52 2026 BST"   (day before month)
// We try each in turn — if none match, the raw string is passed through so
// the frontend can still display something rather than losing the data.
var multipassCreatedLayouts = []string{
	"Mon Jan _2 15:04:05 2006 MST", // ANSIC + zone, month-before-day
	"Mon 02 Jan 15:04:05 2006 MST", // day-before-month
	"Mon 02 Jan 2006 15:04:05 MST",
	time.RFC3339,
}

// knownZoneOffsets covers common IANA zone abbreviations in seconds east of UTC.
// time.Parse only resolves abbreviations present in the host's local tzdata —
// e.g. a UTC-only Linux server parsing "BST" gets offset 0, silently dropping
// the hour adjustment. We consult this table when Parse returns a fabricated
// zero-offset zone so timestamps are deterministic across hosts.
var knownZoneOffsets = map[string]int{
	"UTC": 0, "GMT": 0, "Z": 0,
	"BST": 3600, "IST": 3600, "WEST": 3600, "WET": 0,
	"CET": 3600, "CEST": 7200,
	"EET": 7200, "EEST": 10800,
	"MSK": 10800,
	"EST": -18000, "EDT": -14400,
	"CST": -21600, "CDT": -18000,
	"MST": -25200, "MDT": -21600,
	"PST": -28800, "PDT": -25200,
	"AKST": -32400, "AKDT": -28800,
	"HST": -36000,
	"AEST": 36000, "AEDT": 39600,
	"JST": 32400, "KST": 32400,
	"SGT": 28800, "HKT": 28800,
}

func parseMultipassCreated(raw string) string {
	if raw == "" {
		return ""
	}
	for _, layout := range multipassCreatedLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			name, offset := t.Zone()
			if offset == 0 && name != "UTC" && name != "" {
				if real, ok := knownZoneOffsets[name]; ok {
					t = t.Add(-time.Duration(real) * time.Second)
				}
			}
			return t.UTC().Format(time.RFC3339)
		}
	}
	// Unknown format — surface raw so the UI can display it verbatim rather
	// than losing the data entirely.
	return raw
}

// ListSnapshots returns snapshots for a specific VM, including creation
// timestamps and child arrays. Uses multipass info --snapshots (richer than
// list --snapshots, which only returns parent + comment).
func (c *Client) ListSnapshots(vmName string) ([]SnapshotInfo, error) {
	if err := ValidateVMName(vmName); err != nil {
		return nil, err
	}
	output, err := c.run("info", vmName, "--snapshots", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}

	var resp snapshotInfoJSONResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse snapshots: %w", err)
	}

	vm, ok := resp.Info[vmName]
	if !ok || len(vm.Snapshots) == 0 {
		return []SnapshotInfo{}, nil
	}

	snapshots := make([]SnapshotInfo, 0, len(vm.Snapshots))
	for name, entry := range vm.Snapshots {
		snapshots = append(snapshots, SnapshotInfo{
			Instance: vmName,
			Name:     name,
			Parent:   entry.Parent,
			Comment:  entry.Comment,
			Created:  parseMultipassCreated(entry.Created),
			Children: entry.Children,
		})
	}
	return snapshots, nil
}

// CreateSnapshot creates a named snapshot of a VM.
func (c *Client) CreateSnapshot(vmName, snapshotName, comment string) error {
	if err := ValidateVMName(vmName); err != nil {
		return err
	}
	if err := ValidateVMName(snapshotName); err != nil {
		return fmt.Errorf("invalid snapshot name: %w", err)
	}
	args := []string{"snapshot", "--name", snapshotName}
	if comment != "" {
		args = append(args, "--comment", comment)
	}
	args = append(args, vmName)
	_, err := c.run(args...)
	return err
}

// RestoreSnapshot restores a VM to a snapshot (destructive).
func (c *Client) RestoreSnapshot(vmName, snapshotName string) error {
	if err := ValidateVMName(vmName); err != nil {
		return err
	}
	if err := ValidateVMName(snapshotName); err != nil {
		return fmt.Errorf("invalid snapshot name: %w", err)
	}
	snapshotID := vmName + "." + snapshotName
	_, err := c.run("restore", "--destructive", snapshotID)
	return err
}

// DeleteSnapshot deletes a snapshot.
func (c *Client) DeleteSnapshot(vmName, snapshotName string) error {
	if err := ValidateVMName(vmName); err != nil {
		return err
	}
	if err := ValidateVMName(snapshotName); err != nil {
		return fmt.Errorf("invalid snapshot name: %w", err)
	}
	snapshotID := vmName + "." + snapshotName
	_, err := c.run("delete", "--purge", snapshotID)
	return err
}
