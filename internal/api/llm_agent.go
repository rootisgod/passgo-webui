package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
			// Remove the non-streamed assistant message and re-call with streaming
			// so the user sees tokens appear progressively
			messages = messages[:len(messages)-1]
			streamCh, err := llmChatStream(ctx, cfg, messages)
			if err != nil {
				// Fallback: send the non-streamed content as-is
				if msg.Content != "" {
					eventCh <- sseEvent{Type: "token", Content: msg.Content}
				}
				return
			}
			for ev := range streamCh {
				if ev.Type == "token" {
					eventCh <- sseEvent{Type: "token", Content: ev.Content}
				}
			}
			return
		}

		// Stream any intermediate text the LLM produced alongside tool calls
		// (e.g. "I'll install microk8s now...") so the user sees progress.
		if msg.Content != "" {
			eventCh <- sseEvent{Type: "token", Content: msg.Content}
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

IMPORTANT: The CURRENT VM STATE below is always authoritative and up-to-date. If the conversation history references VMs that are not listed below, those VMs no longer exist — the user may have created, deleted, or modified VMs outside this chat. Always trust the current state over anything in the conversation history.

RULES:
1. Answer informational queries from the VM state below WITHOUT calling tools.
2. Only use tools when the user explicitly asks you to perform an action.
3. NEVER call delete_vm with purge=true unless the user explicitly says "purge" or "permanently delete".
4. NEVER perform bulk destructive operations (deleting, stopping, or suspending multiple VMs) from a single user message. List exactly which VMs will be affected and ask for confirmation first.
5. When exec_command is used, show the user the exact command that will be executed before running it.
6. Do not chain multiple destructive actions in a single turn — execute one, report the result, and wait for the next instruction.
7. ALWAYS include a brief text explanation alongside your tool calls. Before each step, explain what you are about to do and why (e.g. "Installing microk8s via snap..." or "Configuring kubectl access..."). This is critical — the user sees your text as progress updates during long-running operations. Never call tools silently without explanation.

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
	s.groupMu.Lock()
	groups := s.cfg.Groups
	vmGroups := s.cfg.VMGroups
	s.groupMu.Unlock()

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

	return sb.String()
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
