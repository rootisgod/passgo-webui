package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

const (
	maxAgentIterations      = 50
	maxConversationMessages = 50
)

// makeConfirmID generates a deterministic confirmation ID from tool name + args.
// This ensures the ID is stable across LLM retries (which generate new ephemeral tool call IDs).
func makeConfirmID(toolName, argsJSON string) string {
	h := sha256.Sum256([]byte(toolName + ":" + normalizeJSON(argsJSON)))
	return "confirm:" + hex.EncodeToString(h[:8])
}

// makeBulkConfirmID generates a deterministic confirmation ID for a batch of state-changing tools.
func makeBulkConfirmID(toolCalls []toolCall) string {
	var parts []string
	for _, tc := range toolCalls {
		if allowedTools[tc.Function.Name] && !readOnlyTools[tc.Function.Name] {
			parts = append(parts, tc.Function.Name+":"+normalizeJSON(tc.Function.Arguments))
		}
	}
	sort.Strings(parts)
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "bulk:" + hex.EncodeToString(h[:8])
}

// normalizeJSON re-serializes JSON to remove whitespace differences between LLM providers.
func normalizeJSON(s string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return s
	}
	return string(b)
}

type sseEvent struct {
	Type        string `json:"type"`                   // token, tool_start, tool_done, confirm_required, error, done
	Content     string `json:"content,omitempty"`
	Name        string `json:"name,omitempty"`
	Args        string `json:"args,omitempty"`
	Result      string `json:"result,omitempty"`
	ConfirmID   string    `json:"confirm_id,omitempty"`   // unique ID for confirmation flow
	Description string    `json:"description,omitempty"`  // human-readable description of what will happen
	Usage       *llmUsage `json:"usage,omitempty"`        // token usage stats (sent with done event)
}

// runAgentLoop orchestrates the LLM agent: sends messages, executes tool calls,
// and streams the final response via SSE events.
// confirmedTools contains tool call IDs that the user has already approved (for destructive actions).
func (s *Server) runAgentLoop(ctx context.Context, history []chatMessage, confirmedTools map[string]bool, eventCh chan<- sseEvent) {
	var totalUsage llmUsage
	defer func() {
		eventCh <- sseEvent{Type: "done", Usage: &totalUsage}
		close(eventCh)
	}()

	readOnly := s.cfg.LLM.ReadOnly
	tools := filterToolsForMode(readOnly)

	messages := make([]chatMessage, 0, len(history)+2)
	messages = append(messages, chatMessage{Role: "system", Content: ""}) // placeholder, refreshed each iteration
	messages = append(messages, history...)

	cfg := s.cfg.LLM

	writeIterations := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Refresh system prompt with current VM/group state before every LLM call
		// so the model never works from stale data
		messages[0].Content = s.buildSystemPrompt()

		// Trim to keep conversation manageable
		messages = trimMessages(messages, maxConversationMessages)

		// Non-streaming call (handles tool calls)
		msg, usage, err := llmChat(ctx, cfg, messages, tools)
		if usage != nil {
			totalUsage.PromptTokens += usage.PromptTokens
			totalUsage.CompletionTokens += usage.CompletionTokens
			totalUsage.TotalTokens += usage.TotalTokens
		}
		if err != nil {
			s.logger.Error("LLM call failed", "err", err)
			eventCh <- sseEvent{Type: "error", Content: fmt.Sprintf("LLM error: %s", err.Error())}
			return
		}

		// Append assistant response to conversation
		messages = append(messages, *msg)

		// If no tool calls, this is the final text response — re-stream it
		if len(msg.ToolCalls) == 0 {
			// If the non-streamed response already has content, try to re-stream
			// for progressive display. If not, fall back to the non-streamed content.
			if msg.Content != "" {
				messages = messages[:len(messages)-1]
				streamCh, err := llmChatStream(ctx, cfg, messages)
				if err != nil {
					// Fallback: send the non-streamed content as-is
					eventCh <- sseEvent{Type: "token", Content: msg.Content}
					return
				}
				hasContent := false
				for ev := range streamCh {
					if ev.Type == "token" {
						eventCh <- sseEvent{Type: "token", Content: ev.Content}
						hasContent = true
					}
				}
				// If streaming produced nothing, send the non-streamed content
				if !hasContent {
					eventCh <- sseEvent{Type: "token", Content: msg.Content}
				}
			} else {
				// LLM returned empty content with no tool calls — may happen after
				// errors or context exhaustion. Send an informative fallback.
				s.logger.Warn("LLM returned empty response with no tool calls")
				eventCh <- sseEvent{Type: "token", Content: "I encountered an issue processing that request. Please try again or start a new conversation if the context has grown too large."}
			}
			return
		}

		// Stream any intermediate text the LLM produced alongside tool calls
		// (e.g. "I'll install microk8s now...") so the user sees progress.
		// Trim to avoid injecting leading/trailing blank lines into the chat.
		if trimmed := strings.TrimSpace(msg.Content); trimmed != "" {
			eventCh <- sseEvent{Type: "token", Content: trimmed + "\n"}
		}

		// Bulk operation detection: if 2+ state-changing tools in one response,
		// require user confirmation before executing any of them.
		stateChangingCount := 0
		for _, tc := range msg.ToolCalls {
			if allowedTools[tc.Function.Name] && !readOnlyTools[tc.Function.Name] {
				stateChangingCount++
			}
		}
		bulkConfirmID := makeBulkConfirmID(msg.ToolCalls)
		bulkConfirmed := confirmedTools[bulkConfirmID]
		if stateChangingCount >= 2 && !bulkConfirmed {
			desc := describeBulkOperation(msg.ToolCalls)
			s.logger.Info("bulk operation needs confirmation",
				"count", stateChangingCount,
				"description", desc,
			)
			eventCh <- sseEvent{
				Type:        "confirm_required",
				Name:        "bulk_operation",
				ConfirmID:   bulkConfirmID,
				Description: desc,
			}
			// Tell the LLM all tools are pending approval
			for _, tc := range msg.ToolCalls {
				messages = append(messages, chatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    `{"status":"pending_confirmation","message":"Waiting for user to confirm this bulk operation."}`,
				})
			}
			continue
		}

		// Execute each tool call
		for _, tc := range msg.ToolCalls {
			select {
			case <-ctx.Done():
				return
			default:
			}

			eventCh <- sseEvent{
				Type: "tool_start",
				Name: tc.Function.Name,
				Args: tc.Function.Arguments,
			}

			if !allowedTools[tc.Function.Name] {
				errMsg := fmt.Sprintf("Unknown tool: %s", tc.Function.Name)
				s.logger.Warn("LLM tried unknown tool", "tool", tc.Function.Name)
				eventCh <- sseEvent{Type: "tool_done", Name: tc.Function.Name, Result: errMsg}
				messages = append(messages, chatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf(`{"error":"%s"}`, errMsg),
				})
				continue
			}

			// Block write tools in read-only mode (belt-and-suspenders — LLM shouldn't
			// see them, but models can hallucinate tool names)
			if readOnly && !readOnlyTools[tc.Function.Name] {
				errMsg := "Action blocked: chat is in read-only mode"
				s.logger.Warn("read-only mode blocked tool", "tool", tc.Function.Name)
				eventCh <- sseEvent{Type: "tool_done", Name: tc.Function.Name, Result: errMsg}
				messages = append(messages, chatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf(`{"error":"%s"}`, errMsg),
				})
				continue
			}

			// Destructive action confirmation gate
			confirmID := makeConfirmID(tc.Function.Name, tc.Function.Arguments)
			if destructiveTools[tc.Function.Name] && !confirmedTools[confirmID] {
				desc := describeDestructiveAction(tc.Function.Name, tc.Function.Arguments)
				s.logger.Info("destructive action needs confirmation",
					"tool", tc.Function.Name,
					"args", tc.Function.Arguments,
				)
				eventCh <- sseEvent{
					Type:        "confirm_required",
					Name:        tc.Function.Name,
					Args:        tc.Function.Arguments,
					ConfirmID:   confirmID,
					Description: desc,
				}
				// Tell the LLM the action is pending user approval
				messages = append(messages, chatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    `{"status":"pending_confirmation","message":"Waiting for user to confirm this destructive action."}`,
				})
				continue
			}

			// Audit log before execution
			s.logger.Info("executing tool",
				"tool", tc.Function.Name,
				"args", tc.Function.Arguments,
			)

			// Progress callback sends tool_progress SSE events for long-running tools
			progress := func(line string) {
				eventCh <- sseEvent{
					Type:    "tool_progress",
					Name:    tc.Function.Name,
					Content: line,
				}
			}
			result, err := s.executeToolWithProgress(tc.Function.Name, tc.Function.Arguments, progress)
			if err != nil {
				result = fmt.Sprintf(`{"error":"%s"}`, err.Error())
			}

			// Audit log after execution
			s.logger.Info("tool completed",
				"tool", tc.Function.Name,
				"success", err == nil,
				"result_preview", truncate(result, 200),
			)

			// Emit to event log for non-read-only tools
			if !readOnlyTools[tc.Function.Name] {
				evResult := "success"
				evDetail := ""
				if err != nil {
					evResult = "failed"
					evDetail = err.Error()
				}
				resource := extractToolResource(tc.Function.Name, tc.Function.Arguments)
				s.eventLog.EmitEvent("llm", tc.Function.Name, "llm_agent", resource, evResult, evDetail)
			}

			eventCh <- sseEvent{
				Type:   "tool_done",
				Name:   tc.Function.Name,
				Result: truncate(result, 2000),
			}

			messages = append(messages, chatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})

			// Only count write tools against the iteration limit
			if !readOnlyTools[tc.Function.Name] {
				writeIterations++
			}
		}

		if writeIterations >= maxAgentIterations {
			eventCh <- sseEvent{Type: "error", Content: fmt.Sprintf("Safety limit reached (%d write operations)", maxAgentIterations)}
			return
		}
		// Loop continues — send tool results back to LLM
	}
}

// describeDestructiveAction returns a human-readable description of what a
// destructive tool call will do.
func describeDestructiveAction(toolName, argsJSON string) string {
	switch toolName {
	case "delete_vm":
		var args struct {
			Name  string `json:"name"`
			Purge bool   `json:"purge"`
		}
		json.Unmarshal([]byte(argsJSON), &args)
		if args.Purge {
			return fmt.Sprintf("Permanently purge VM '%s' (cannot be recovered)", args.Name)
		}
		return fmt.Sprintf("Delete VM '%s'", args.Name)
	case "restore_snapshot":
		var args struct {
			VM       string `json:"vm"`
			Snapshot string `json:"snapshot"`
		}
		json.Unmarshal([]byte(argsJSON), &args)
		return fmt.Sprintf("Restore VM '%s' to snapshot '%s' (current state will be lost)", args.VM, args.Snapshot)
	case "delete_snapshot":
		var args struct {
			VM       string `json:"vm"`
			Snapshot string `json:"snapshot"`
		}
		json.Unmarshal([]byte(argsJSON), &args)
		return fmt.Sprintf("Delete snapshot '%s' from VM '%s'", args.Snapshot, args.VM)
	default:
		return fmt.Sprintf("Execute %s", toolName)
	}
}

// describeBulkOperation returns a human-readable description of all
// state-changing tool calls in a batch.
func describeBulkOperation(toolCalls []toolCall) string {
	var ops []string
	for _, tc := range toolCalls {
		if readOnlyTools[tc.Function.Name] {
			continue
		}
		var args struct {
			Name     string `json:"name"`
			VM       string `json:"vm"`
			Snapshot string `json:"snapshot"`
			Purge    bool   `json:"purge"`
		}
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		target := args.Name
		if target == "" {
			target = args.VM
		}
		switch tc.Function.Name {
		case "stop_vm":
			ops = append(ops, fmt.Sprintf("Stop '%s'", target))
		case "start_vm":
			ops = append(ops, fmt.Sprintf("Start '%s'", target))
		case "suspend_vm":
			ops = append(ops, fmt.Sprintf("Suspend '%s'", target))
		case "delete_vm":
			if args.Purge {
				ops = append(ops, fmt.Sprintf("Purge '%s'", target))
			} else {
				ops = append(ops, fmt.Sprintf("Delete '%s'", target))
			}
		case "recover_vm":
			ops = append(ops, fmt.Sprintf("Recover '%s'", target))
		case "create_snapshot":
			ops = append(ops, fmt.Sprintf("Snapshot '%s'", target))
		case "restore_snapshot":
			ops = append(ops, fmt.Sprintf("Restore '%s' to snapshot '%s'", target, args.Snapshot))
		case "delete_snapshot":
			ops = append(ops, fmt.Sprintf("Delete snapshot '%s' from '%s'", args.Snapshot, target))
		default:
			ops = append(ops, fmt.Sprintf("%s on '%s'", tc.Function.Name, target))
		}
	}
	return fmt.Sprintf("Bulk operation (%d actions): %s", len(ops), strings.Join(ops, ", "))
}

// buildSystemPrompt creates a system message with current VM inventory.
func (s *Server) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString(`You are an AI assistant for managing Multipass virtual machines via PassGo Web.
Keep responses concise and helpful.

YOUR TOOLS:
- VM lifecycle: list_vms, get_vm_info, create_vm, start_vm, stop_vm, suspend_vm, delete_vm, recover_vm
- Snapshots: list_snapshots, create_snapshot, restore_snapshot, delete_snapshot
- Execution: exec_command (run commands inside a VM)
- Networks: list_networks
- Groups: list_groups, create_group, rename_group, delete_group, assign_vm_to_group (organize VMs into named groups)
- Cloud-Init: list_cloud_init_templates, get_cloud_init_template, create_cloud_init_template, update_cloud_init_template, delete_cloud_init_template (manage cloud-init templates for VM provisioning)
- Ansible: list_playbooks, get_playbook, create_playbook, update_playbook, delete_playbook (manage Ansible playbooks for VM configuration)

IMPORTANT: The CURRENT VM STATE below is always authoritative and up-to-date. If the conversation history references VMs that are not listed below, those VMs no longer exist — the user may have created, deleted, or modified VMs outside this chat. Always trust the current state over anything in the conversation history.

RULES:
1. Answer informational queries from the VM state below WITHOUT calling tools.
2. Only use tools when the user explicitly asks you to perform an action.
3. NEVER call delete_vm with purge=true unless the user explicitly says "purge" or "permanently delete".
4. NEVER perform bulk destructive operations (deleting, stopping, or suspending multiple VMs) from a single user message. List exactly which VMs will be affected and ask for confirmation first.
5. When exec_command is used, show the user the exact command that will be executed before running it.
6. Do not chain multiple destructive actions in a single turn — execute one, report the result, and wait for the next instruction.
7. ALWAYS include a brief text explanation alongside your tool calls. Before each step, explain what you are about to do and why (e.g. "Installing microk8s via snap..." or "Configuring kubectl access..."). This is critical — the user sees your text as progress updates during long-running operations. Never call tools silently without explanation.
8. CLOUD-INIT BEST PRACTICES (these VMs are always Ubuntu on Multipass):
   YAML FORMAT:
   - Content must start with "#cloud-config" on the first line. Filenames must end in .yaml or .yml.
   - Always quote string values containing colons, exclamation marks, or special characters. Use block scalars (|) for multi-line strings in runcmd.
   - YAML gotcha: "yes", "no", "true", "false", "on", "off" are interpreted as booleans — quote them if meant as strings.

   WORKFLOW:
   - When the user asks to create a VM with software, ALWAYS create a cloud-init template first, then use create_vm with the cloud_init parameter. Never manually install software via exec_command when cloud-init can do it.
   - After creating a VM with cloud-init, tell the user they can check progress on the Summary tab (cloud-init status) or via exec_command: cloud-init status --wait.

   UBUNTU/MULTIPASS SPECIFICS:
   - The default user is 'ubuntu' with home at /home/ubuntu. cloud-init runs as root.
   - For user-specific operations (dotfiles, aliases, permissions), target 'ubuntu' explicitly (chown ubuntu:ubuntu).
   - Use 'package_update: true' and 'package_upgrade: true' instead of 'apt-get update/upgrade' in runcmd.
   - Use the 'packages' list for apt packages — cloud-init handles retries and locking. Don't use apt in runcmd unless you need specific flags.
   - For PPAs or third-party repos, use 'apt' sources config (apt: sources:) rather than add-apt-repository in runcmd.
   - Use 'snap' module for snap installs where possible. In runcmd, add 'snap wait system seed.loaded' before any snap commands.
   - /etc/profile.d/*.sh scripts are sourced on login — use write_files to drop PATH or alias scripts there.
   - Group membership changes (usermod -aG) require logout/login. Note this to the user.
   - Multipass VMs have internet access by default. No proxy config needed unless user specifies.

   CLOUD-INIT MODULES (preferred order in the YAML):
   - package_update, package_upgrade (top) — run apt update/upgrade
   - packages — list of apt packages to install
   - snap — snap packages (e.g. {name: go, channel: latest/stable, classic: true})
   - write_files — create config files, scripts, aliases BEFORE runcmd runs
   - runcmd — shell commands, run in order, each as root. Use ["bash", "-c", "..."] for pipes/redirects.
   - final_message — confirmation string (always quote it)

   COMMON PATTERNS:
   - write_files for /etc/profile.d/aliases.sh is better than echo >> .bashrc in runcmd.
   - For services that need enabling: use 'systemctl enable --now <service>' in runcmd.
   - For downloading binaries: use 'curl -fsSL <url> -o /usr/local/bin/<name> && chmod +x /usr/local/bin/<name>'.
   - For docker: package 'docker.io' and 'usermod -aG docker ubuntu' in runcmd. Remind user to re-login.
   - For microk8s: snap install, 'microk8s status --wait-ready', enable addons ONE AT A TIME, then configure kubectl.

9. ANSIBLE PLAYBOOK BEST PRACTICES:
   WHEN TO USE:
   - Use Ansible playbooks when the user wants repeatable, multi-step configuration that can be re-run.
   - Ansible is better than cloud-init for: iterative development, multi-VM orchestration, idempotent configuration, and tasks that need debugging.
   - Cloud-init is better for: one-time bootstrap on first boot. Suggest Ansible when the task involves ongoing configuration management.

   PLAYBOOK FORMAT:
   - Standard Ansible YAML. Must be a list of plays. Each play needs 'hosts' and 'tasks' keys at minimum.
   - Always set 'hosts: all' — the inventory controls which VMs are targeted. The user selects targets in the Ansible tab.
   - Use 'become: true' for tasks that need root (package installs, service management, file writes outside /home).
   - Use Ansible modules (apt, copy, service, template, file, user) instead of raw shell commands where possible — they're idempotent.

   STRUCTURE:
   - Keep playbooks focused on one purpose (e.g. 'install-nginx.yml', 'setup-monitoring.yml').
   - Use descriptive task names — they show up in the execution output.
   - Group related tasks logically. Use handlers for service restarts.

   COMMON PATTERNS:
   - Package install: apt module with state=present and update_cache=yes
   - File creation: copy module with content parameter, or template for dynamic content
   - Service management: systemd module with state=started and enabled=yes
   - User management: user module for creating users, authorized_key module for SSH keys
   - Docker: install via apt (docker.io), add user to docker group, restart docker service

   CRITICAL — NEVER DO THESE IN PLAYBOOKS:
   - NEVER use 'newgrp' — it opens an interactive shell and hangs Ansible permanently. Group changes take effect on next login; tell the user to reconnect.
   - NEVER use commands that expect interactive input (passwd without stdin, apt prompts without -y, read, etc.).
   - NEVER use 'sudo su -' or 'su -' — use 'become: true' instead.
   - NEVER use 'reboot' command directly — use the 'reboot' module which waits for the host to come back.
   - NEVER use 'systemctl restart' in a task for services you just installed — use handlers with 'notify' so restarts only happen when changes are made.
   - AVOID shell/command modules when an Ansible module exists (e.g. use 'apt' not 'apt-get', 'user' not 'useradd', 'copy' not 'cp').
   - AVOID 'ignore_errors: yes' as a default — only use it when you genuinely expect and handle the error case.

   WORKFLOW:
   - When the user asks you to create a playbook, use create_playbook to save it. Tell them to go to the Ansible tab on the VM page to run it.
   - When the user asks to modify a playbook, use get_playbook to read it first, then update_playbook.

`)


	if s.cfg.LLM.ReadOnly {
		sb.WriteString("MODE: READ-ONLY. You can only view information. All state-changing actions are disabled.\n\n")
	}

	vms, err := s.mp.ListVMs()
	if err != nil {
		sb.WriteString("CURRENT VM STATE: Unable to fetch VMs: " + err.Error() + "\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("CURRENT VM STATE (%d instances):\n", len(vms)))
	if len(vms) == 0 {
		sb.WriteString("No virtual machines found.\n")
	} else {
		for _, vm := range vms {
			line := fmt.Sprintf("- %s: state=%s", vm.Name, vm.State)
			if len(vm.IPv4) > 0 && vm.IPv4[0] != "" && vm.IPv4[0] != "--" {
				line += fmt.Sprintf(", ip=%s", vm.IPv4[0])
			}
			if vm.CPUs != "" {
				line += fmt.Sprintf(", cpus=%s", vm.CPUs)
			}
			if vm.MemoryTotal != "" {
				line += fmt.Sprintf(", memory=%s/%s", vm.MemoryUsage, vm.MemoryTotal)
			}
			if vm.DiskTotal != "" {
				line += fmt.Sprintf(", disk=%s/%s", vm.DiskUsage, vm.DiskTotal)
			}
			if vm.Release != "" {
				line += fmt.Sprintf(", release=%s", vm.Release)
			}
			sb.WriteString(line + "\n")
		}
	}

	// Include group information
	s.cfgMu.Lock()
	groups := s.cfg.Groups
	vmGroups := s.cfg.VMGroups
	s.cfgMu.Unlock()

	if len(groups) > 0 {
		sb.WriteString(fmt.Sprintf("\nGROUPS (%d):\n", len(groups)))
		for _, g := range groups {
			var members []string
			for vm, grp := range vmGroups {
				if grp == g {
					members = append(members, vm)
				}
			}
			if len(members) > 0 {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", g, strings.Join(members, ", ")))
			} else {
				sb.WriteString(fmt.Sprintf("- %s: (empty)\n", g))
			}
		}
	}

	// Include cloud-init template list
	var dirs []string
	if s.cfg.CloudInitDir != "" {
		dirs = append(dirs, s.cfg.CloudInitDir)
	}
	templates, _ := s.mp.GetAllCloudInitTemplates(dirs)
	entries, _ := s.builtinTemplatesFS.ReadDir("cloud-init")
	for _, entry := range entries {
		if !entry.IsDir() {
			templates = append(templates, multipass.TemplateOption{Label: entry.Name(), BuiltIn: true})
		}
	}
	if len(templates) > 0 {
		sb.WriteString(fmt.Sprintf("\nCLOUD-INIT TEMPLATES (%d):\n", len(templates)))
		for _, t := range templates {
			if t.BuiltIn {
				sb.WriteString(fmt.Sprintf("- %s (built-in)\n", t.Label))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", t.Label))
			}
		}
	}

	// Include ansible playbook list
	playbookNames, _ := multipass.ListPlaybooks(s.cfg.PlaybooksDir)
	if len(playbookNames) > 0 {
		sb.WriteString(fmt.Sprintf("\nANSIBLE PLAYBOOKS (%d):\n", len(playbookNames)))
		for _, name := range playbookNames {
			sb.WriteString(fmt.Sprintf("- %s\n", name))
		}
	}

	return sb.String()
}

// extractToolResource pulls the primary resource name from a tool call's JSON args.
func extractToolResource(toolName, argsJSON string) string {
	var args struct {
		Name     string `json:"name"`
		VM       string `json:"vm"`
		Snapshot string `json:"snapshot"`
	}
	json.Unmarshal([]byte(argsJSON), &args)
	if args.Name != "" {
		return args.Name
	}
	if args.VM != "" {
		return args.VM
	}
	return toolName
}

// trimMessages keeps the conversation within limits by preserving the system
// prompt and the most recent messages.
func trimMessages(messages []chatMessage, maxCount int) []chatMessage {
	if len(messages) <= maxCount {
		return messages
	}
	// Keep system prompt (index 0) + most recent messages
	keep := make([]chatMessage, 0, maxCount)
	keep = append(keep, messages[0])
	keep = append(keep, messages[len(messages)-(maxCount-1):]...)
	return keep
}
