package api

import (
	"encoding/json"
	"fmt"
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
		output, err := s.mp.ExecInVM(args.VM, args.Command)
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
