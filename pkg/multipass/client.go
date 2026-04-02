package multipass

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// CommandRunner executes a multipass CLI command and returns stdout.
type CommandRunner func(args ...string) (string, error)

// Client wraps the multipass CLI.
type Client struct {
	logger *slog.Logger
	run    CommandRunner
}

// NewClient creates a Client that calls the real multipass binary.
func NewClient(logger *slog.Logger) *Client {
	c := &Client{logger: logger}
	c.run = c.defaultRunner
	return c
}

// NewClientWithRunner creates a Client with a custom command runner (for testing).
func NewClientWithRunner(logger *slog.Logger, runner CommandRunner) *Client {
	return &Client{logger: logger, run: runner}
}

func (c *Client) defaultRunner(args ...string) (string, error) {
	cmd := exec.Command("multipass", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	c.logger.Debug("exec", "cmd", "multipass "+strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		c.logger.Error("exec failed", "cmd", "multipass "+strings.Join(args, " "), "err", err, "stderr", errMsg)
		return "", fmt.Errorf("command failed: %w\nStderr: %s", err, errMsg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// runWithContext executes a multipass command with a context for cancellation/timeout.
func (c *Client) runWithContext(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "multipass", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	c.logger.Debug("exec", "cmd", "multipass "+strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("command timed out after deadline exceeded")
		}
		errMsg := strings.TrimSpace(stderr.String())
		c.logger.Error("exec failed", "cmd", "multipass "+strings.Join(args, " "), "err", err, "stderr", errMsg)
		return "", fmt.Errorf("command failed: %w\nStderr: %s", err, errMsg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RandomVMName generates a name like "VM-a1b2".
func RandomVMName() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, VMNameRandomLength)
	if _, err := rand.Read(b); err != nil {
		return VMNamePrefix + "0000"
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return VMNamePrefix + string(b)
}
