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

func parseMultipassCreated(raw string) string {
	if raw == "" {
		return ""
	}
	for _, layout := range multipassCreatedLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
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
