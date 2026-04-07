package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

// progressFn is called during long-running tool execution to send progress updates.
type progressFn func(line string)

// executeTool dispatches a tool call to the corresponding multipass.Client method.
// Tool-level errors are returned as JSON strings (not Go errors) so the LLM can
// explain failures to the user. Only truly unexpected errors return as Go errors.
func (s *Server) executeTool(toolName string, argsJSON string) (string, error) {
	return s.executeToolWithProgress(toolName, argsJSON, nil)
}

// executeToolWithProgress is like executeTool but accepts a progress callback
// for long-running operations (exec_command, create_vm).
func (s *Server) executeToolWithProgress(toolName string, argsJSON string, progress progressFn) (string, error) {
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
			Name      string `json:"name"`
			Image     string `json:"image"`
			CPUs      int    `json:"cpus"`
			MemoryMB  int    `json:"memory_mb"`
			DiskGB    int    `json:"disk_gb"`
			CloudInit string `json:"cloud_init"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}

		// Resolve cloud-init template name to a file path
		var cloudInitFile string
		var tmpCloudInit string
		if args.CloudInit != "" {
			// Check built-in templates first
			if data, err := s.builtinTemplatesFS.ReadFile("cloud-init/" + args.CloudInit); err == nil {
				tmp := filepath.Join(os.TempDir(), "passgo-cloudinit-"+args.CloudInit)
				if err := os.WriteFile(tmp, data, 0600); err != nil {
					return toolError(fmt.Errorf("failed to prepare cloud-init template: %w", err)), nil
				}
				cloudInitFile = tmp
				tmpCloudInit = tmp
			} else if s.cfg.CloudInitDir != "" {
				// Try user templates
				content, err := multipass.ReadCloudInitTemplate(s.cfg.CloudInitDir, args.CloudInit)
				if err != nil {
					return toolError(fmt.Errorf("cloud-init template '%s' not found", args.CloudInit)), nil
				}
				tmp := filepath.Join(os.TempDir(), "passgo-cloudinit-"+args.CloudInit)
				if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
					return toolError(fmt.Errorf("failed to prepare cloud-init template: %w", err)), nil
				}
				cloudInitFile = tmp
				tmpCloudInit = tmp
			} else {
				return toolError(fmt.Errorf("cloud-init template '%s' not found", args.CloudInit)), nil
			}
		}

		// Launch asynchronously and poll for completion, sending progress updates
		vmName := args.Name
		if vmName == "" {
			vmName = multipass.RandomVMName()
		}
		type launchResult struct {
			name string
			err  error
		}
		done := make(chan launchResult, 1)
		go func() {
			name, err := s.mp.LaunchVM(args.Name, args.Image, args.CPUs, args.MemoryMB, args.DiskGB, cloudInitFile, "")
			// Clean up temp file
			if tmpCloudInit != "" {
				os.Remove(tmpCloudInit)
			}
			done <- launchResult{name, err}
		}()
		// Poll with progress updates every 5 seconds
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case res := <-done:
				if res.err != nil {
					return toolError(res.err), nil
				}
				return fmt.Sprintf(`{"status":"created","vm":"%s"}`, res.name), nil
			case <-ticker.C:
				if progress != nil {
					progress(fmt.Sprintf("Still creating VM '%s'...", vmName))
				}
			}
		}

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
		output, err := s.mp.ExecInVMStreaming(ctx, args.VM, args.Command, progress)
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

	case "list_cloud_init_templates":
		var dirs []string
		if s.cfg.CloudInitDir != "" {
			dirs = append(dirs, s.cfg.CloudInitDir)
		}
		templates, err := s.mp.GetAllCloudInitTemplates(dirs)
		if err != nil {
			templates = nil
		}
		// Merge built-in templates
		entries, _ := s.builtinTemplatesFS.ReadDir("cloud-init")
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			templates = append(templates, multipass.TemplateOption{
				Label:   entry.Name(),
				Path:    "builtin:" + entry.Name(),
				BuiltIn: true,
			})
		}
		if templates == nil {
			templates = []multipass.TemplateOption{}
		}
		return toJSON(templates), nil

	case "get_cloud_init_template":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		// Check built-in templates first
		if data, err := s.builtinTemplatesFS.ReadFile("cloud-init/" + args.Name); err == nil {
			return toJSON(map[string]any{"name": args.Name, "content": string(data), "builtIn": true}), nil
		}
		if s.cfg.CloudInitDir == "" {
			return toolError(fmt.Errorf("template '%s' not found", args.Name)), nil
		}
		content, err := multipass.ReadCloudInitTemplate(s.cfg.CloudInitDir, args.Name)
		if err != nil {
			return toolError(err), nil
		}
		return toJSON(map[string]any{"name": args.Name, "content": content, "builtIn": false}), nil

	case "create_cloud_init_template":
		var args struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if s.cfg.CloudInitDir == "" {
			return toolError(fmt.Errorf("cloud-init directory not configured")), nil
		}
		if args.Name == "" || args.Content == "" {
			return toolError(fmt.Errorf("name and content are required")), nil
		}
		if err := multipass.ValidateCloudInitYAML(args.Content); err != nil {
			return toolError(fmt.Errorf("invalid cloud-init content: %w", err)), nil
		}
		// Check if already exists
		if _, err := multipass.ReadCloudInitTemplate(s.cfg.CloudInitDir, args.Name); err == nil {
			return toolError(fmt.Errorf("template '%s' already exists", args.Name)), nil
		}
		if err := multipass.WriteCloudInitTemplate(s.cfg.CloudInitDir, args.Name, args.Content); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"created","template":"%s"}`, args.Name), nil

	case "update_cloud_init_template":
		var args struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if s.cfg.CloudInitDir == "" {
			return toolError(fmt.Errorf("cloud-init directory not configured")), nil
		}
		if args.Content == "" {
			return toolError(fmt.Errorf("content is required")), nil
		}
		if err := multipass.ValidateCloudInitYAML(args.Content); err != nil {
			return toolError(fmt.Errorf("invalid cloud-init content: %w", err)), nil
		}
		// Verify it exists
		if _, err := multipass.ReadCloudInitTemplate(s.cfg.CloudInitDir, args.Name); err != nil {
			return toolError(fmt.Errorf("template '%s' not found", args.Name)), nil
		}
		if err := multipass.WriteCloudInitTemplate(s.cfg.CloudInitDir, args.Name, args.Content); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"updated","template":"%s"}`, args.Name), nil

	case "delete_cloud_init_template":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if s.cfg.CloudInitDir == "" {
			return toolError(fmt.Errorf("cloud-init directory not configured")), nil
		}
		if err := multipass.DeleteCloudInitTemplate(s.cfg.CloudInitDir, args.Name); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"deleted","template":"%s"}`, args.Name), nil

	case "list_playbooks":
		names, err := multipass.ListPlaybooks(s.cfg.PlaybooksDir)
		if err != nil {
			return toolError(err), nil
		}
		type entry struct {
			Name string `json:"name"`
		}
		entries := make([]entry, len(names))
		for i, n := range names {
			entries[i] = entry{Name: n}
		}
		return toJSON(entries), nil

	case "get_playbook":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		content, err := multipass.ReadPlaybook(s.cfg.PlaybooksDir, args.Name)
		if err != nil {
			return toolError(err), nil
		}
		return toJSON(map[string]string{"name": args.Name, "content": content}), nil

	case "create_playbook":
		var args struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if args.Name == "" || args.Content == "" {
			return toolError(fmt.Errorf("name and content are required")), nil
		}
		if _, err := multipass.ReadPlaybook(s.cfg.PlaybooksDir, args.Name); err == nil {
			return toolError(fmt.Errorf("playbook '%s' already exists", args.Name)), nil
		}
		if err := multipass.WritePlaybook(s.cfg.PlaybooksDir, args.Name, args.Content); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"created","playbook":"%s"}`, args.Name), nil

	case "update_playbook":
		var args struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if args.Content == "" {
			return toolError(fmt.Errorf("content is required")), nil
		}
		if _, err := multipass.ReadPlaybook(s.cfg.PlaybooksDir, args.Name); err != nil {
			return toolError(fmt.Errorf("playbook '%s' not found", args.Name)), nil
		}
		if err := multipass.WritePlaybook(s.cfg.PlaybooksDir, args.Name, args.Content); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"updated","playbook":"%s"}`, args.Name), nil

	case "delete_playbook":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
		}
		if err := multipass.DeletePlaybook(s.cfg.PlaybooksDir, args.Name); err != nil {
			return toolError(err), nil
		}
		return fmt.Sprintf(`{"status":"deleted","playbook":"%s"}`, args.Name), nil

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
