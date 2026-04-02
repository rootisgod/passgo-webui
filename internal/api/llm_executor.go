package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

// executeTool dispatches a tool call to the corresponding multipass.Client method.
// Tool-level errors are returned as JSON strings (not Go errors) so the LLM can
// explain failures to the user. Only truly unexpected errors return as Go errors.
func (s *Server) executeTool(toolName string, argsJSON string) (string, error) {
	if !allowedTools[toolName] {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	switch toolName {
	case "list_vms":
		vms, err := s.mp.ListVMs()
		if err != nil {
			return toolError(err), nil
		}
		return toJSON(vms), nil

	case "get_vm_info":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		vm, err := s.mp.GetVMInfo(args.Name)
		if err != nil {
			return toolError(err), nil
		}
		return toJSON(vm), nil

	case "start_vm":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.StartVM(args.Name); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"started","vm":"%s"}`, args.Name), nil

	case "stop_vm":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.StopVM(args.Name); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"stopped","vm":"%s"}`, args.Name), nil

	case "suspend_vm":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.SuspendVM(args.Name); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"suspended","vm":"%s"}`, args.Name), nil

	case "delete_vm":
		var args struct {
			Name  string `json:"name"`
			Purge bool   `json:"purge"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.DeleteVM(args.Name, args.Purge); err != nil {
			return toolError(err), nil
		}
		action := "deleted"
		if args.Purge {
			action = "purged"
		}
		return fmt.Sprintf(`{"status":"%s","vm":"%s"}`, action, args.Name), nil

	case "recover_vm":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.RecoverVM(args.Name); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"recovered","vm":"%s"}`, args.Name), nil

	case "create_vm":
		var args struct {
			Name     string `json:"name"`
			Image    string `json:"image"`
			CPUs     int    `json:"cpus"`
			MemoryMB int    `json:"memory_mb"`
			DiskGB   int    `json:"disk_gb"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		vmName, err := s.mp.LaunchVM(args.Name, args.Image, args.CPUs, args.MemoryMB, args.DiskGB, "", "")
		if err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"created","vm":"%s"}`, vmName), nil

	case "exec_command":
		var args struct {
			VM      string   `json:"vm"`
			Command []string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		output, err := s.mp.ExecInVMWithContext(ctx, args.VM, args.Command)
		if err != nil {
			return toolError(err), nil
		}
		return toJSON(map[string]string{"output": output}), nil

	case "create_snapshot":
		var args struct {
			VM      string `json:"vm"`
			Name    string `json:"name"`
			Comment string `json:"comment"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.CreateSnapshot(args.VM, args.Name, args.Comment); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"created","vm":"%s","snapshot":"%s"}`, args.VM, args.Name), nil

	case "list_snapshots":
		var args struct {
			VM string `json:"vm"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		snaps, err := s.mp.ListSnapshots(args.VM)
		if err != nil {
			return toolError(err), nil
		}
		return toJSON(snaps), nil

	case "restore_snapshot":
		var args struct {
			VM       string `json:"vm"`
			Snapshot string `json:"snapshot"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.RestoreSnapshot(args.VM, args.Snapshot); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"restored","vm":"%s","snapshot":"%s"}`, args.VM, args.Snapshot), nil

	case "delete_snapshot":
		var args struct {
			VM       string `json:"vm"`
			Snapshot string `json:"snapshot"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := s.mp.DeleteSnapshot(args.VM, args.Snapshot); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"deleted","vm":"%s","snapshot":"%s"}`, args.VM, args.Snapshot), nil

	case "list_networks":
		nets, err := s.mp.ListNetworks()
		if err != nil {
			return toolError(err), nil
		}
		return toJSON(nets), nil

	case "list_groups":
		s.groupMu.Lock()
		groups := s.cfg.Groups
		vmGroups := s.cfg.VMGroups
		s.groupMu.Unlock()
		return toJSON(map[string]any{"groups": groups, "vm_groups": vmGroups}), nil

	case "create_group":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		s.groupMu.Lock()
		defer s.groupMu.Unlock()
		if slices.Contains(s.cfg.Groups, args.Name) {
			return toolError(fmt.Errorf("group '%s' already exists", args.Name)), nil
		}
		s.cfg.Groups = append(s.cfg.Groups, args.Name)
		if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"created","group":"%s"}`, args.Name), nil

	case "rename_group":
		var args struct {
			OldName string `json:"old_name"`
			NewName string `json:"new_name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		s.groupMu.Lock()
		defer s.groupMu.Unlock()
		idx := slices.Index(s.cfg.Groups, args.OldName)
		if idx < 0 {
			return toolError(fmt.Errorf("group '%s' not found", args.OldName)), nil
		}
		if args.OldName != args.NewName && slices.Contains(s.cfg.Groups, args.NewName) {
			return toolError(fmt.Errorf("group '%s' already exists", args.NewName)), nil
		}
		s.cfg.Groups[idx] = args.NewName
		for vm, g := range s.cfg.VMGroups {
			if g == args.OldName {
				s.cfg.VMGroups[vm] = args.NewName
			}
		}
		if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"renamed","old_name":"%s","new_name":"%s"}`, args.OldName, args.NewName), nil

	case "delete_group":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		s.groupMu.Lock()
		defer s.groupMu.Unlock()
		idx := slices.Index(s.cfg.Groups, args.Name)
		if idx < 0 {
			return toolError(fmt.Errorf("group '%s' not found", args.Name)), nil
		}
		s.cfg.Groups = slices.Delete(s.cfg.Groups, idx, idx+1)
		for vm, g := range s.cfg.VMGroups {
			if g == args.Name {
				delete(s.cfg.VMGroups, vm)
			}
		}
		if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"deleted","group":"%s"}`, args.Name), nil

	case "assign_vm_to_group":
		var args struct {
			VM    string `json:"vm"`
			Group string `json:"group"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		s.groupMu.Lock()
		defer s.groupMu.Unlock()
		if args.Group != "" && !slices.Contains(s.cfg.Groups, args.Group) {
			return toolError(fmt.Errorf("group '%s' not found", args.Group)), nil
		}
		if args.Group == "" {
			delete(s.cfg.VMGroups, args.VM)
		} else {
			s.cfg.VMGroups[args.VM] = args.Group
		}
		if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
			return toolError(err), nil
		}
		action := "assigned"
		if args.Group == "" {
			action = "unassigned"
		}
		return fmt.Sprintf(`{"status":"%s","vm":"%s","group":"%s"}`, action, args.VM, args.Group), nil

	default:
		return "", fmt.Errorf("unhandled tool: %s", toolName)
	}
}

func toolError(err error) string {
	return fmt.Sprintf(`{"error":"%s"}`, err.Error())
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"marshal error: %s"}`, err.Error())
	}
	return string(b)
}
