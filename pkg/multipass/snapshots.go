package multipass

import (
	"encoding/json"
	"fmt"
)

// snapshotListJSONResponse is the response from multipass list --snapshots --format json.
// Structure: {"errors":[],"info":{"vmName":{"snapName":{"parent":"...","comment":"..."}, ...}}}
type snapshotListJSONResponse struct {
	Errors []string                                    `json:"errors"`
	Info   map[string]map[string]snapshotListJSONEntry `json:"info"`
}

type snapshotListJSONEntry struct {
	Parent  string `json:"parent"`
	Comment string `json:"comment"`
}

// ListSnapshots returns snapshots for a specific VM using multipass list --snapshots.
func (c *Client) ListSnapshots(vmName string) ([]SnapshotInfo, error) {
	if err := ValidateVMName(vmName); err != nil {
		return nil, err
	}
	output, err := c.run("list", "--snapshots", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}

	var resp snapshotListJSONResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse snapshots: %w", err)
	}

	vmSnaps, ok := resp.Info[vmName]
	if !ok {
		// No snapshots for this VM
		return []SnapshotInfo{}, nil
	}

	var snapshots []SnapshotInfo
	for name, entry := range vmSnaps {
		snapshots = append(snapshots, SnapshotInfo{
			Instance: vmName,
			Name:     name,
			Parent:   entry.Parent,
			Comment:  entry.Comment,
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
