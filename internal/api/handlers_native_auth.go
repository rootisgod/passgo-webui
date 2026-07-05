package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"time"
)

const nativeAuthCheckTimeout = 5 * time.Second

type nativeAuthStatusResponse struct {
	Providers []nativeAuthProviderStatus `json:"providers"`
}

type nativeAuthProviderStatus struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Command       string `json:"command"`
	Installed     bool   `json:"installed"`
	Authenticated bool   `json:"authenticated"`
	Status        string `json:"status"`
}

type nativeAuthCommand struct {
	id      string
	label   string
	binary  string
	args    []string
	command string
}

var nativeAuthCommands = []nativeAuthCommand{
	{
		id:      "codex",
		label:   "ChatGPT (Codex CLI)",
		binary:  "codex",
		args:    []string{"login", "status"},
		command: "codex login status",
	},
	{
		id:      "claude",
		label:   "Claude Code",
		binary:  "claude",
		args:    []string{"auth", "status"},
		command: "claude auth status",
	},
}

var (
	nativeAuthLookPath   = exec.LookPath
	nativeAuthRunCommand = runNativeAuthCommand
)

func (s *Server) handleGetNativeAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, nativeAuthStatusResponse{
		Providers: checkNativeAuthStatus(r.Context()),
	})
}

func checkNativeAuthStatus(ctx context.Context) []nativeAuthProviderStatus {
	providers := make([]nativeAuthProviderStatus, 0, len(nativeAuthCommands))
	for _, command := range nativeAuthCommands {
		providers = append(providers, checkNativeAuthProvider(ctx, command))
	}
	return providers
}

func checkNativeAuthProvider(ctx context.Context, command nativeAuthCommand) nativeAuthProviderStatus {
	status := nativeAuthProviderStatus{
		ID:      command.id,
		Label:   command.label,
		Command: command.command,
	}

	path, err := nativeAuthLookPath(command.binary)
	if err != nil {
		status.Status = "not_installed"
		return status
	}
	status.Installed = true

	checkCtx, cancel := context.WithTimeout(ctx, nativeAuthCheckTimeout)
	defer cancel()
	if err := nativeAuthRunCommand(checkCtx, path, command.args...); err != nil {
		if errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
			status.Status = "timeout"
			return status
		}
		status.Status = "not_authenticated"
		return status
	}

	status.Authenticated = true
	status.Status = "authenticated"
	return status
}

func runNativeAuthCommand(ctx context.Context, path string, args ...string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
