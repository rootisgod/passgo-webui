package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/aiaccess"
	"github.com/rootisgod/passgo-web/internal/config"
)

const nativeLLMTimeout = 5 * time.Minute

var (
	nativeLLMLookPath   = exec.LookPath
	nativeLLMRunCommand = runNativeLLMCommand
)

type nativeLLMResponse struct {
	Content   string              `json:"content"`
	ToolCalls []nativeLLMToolCall `json:"tool_calls"`
}

type nativeLLMToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) llmComplete(ctx context.Context, cfg *config.LLMConfig, messages []chatMessage, tools []toolDef) (*chatMessage, *llmUsage, error) {
	if aiaccess.IsNativeProvider(cfg.Provider) {
		msg, err := s.nativeLLMChat(ctx, cfg, messages, tools)
		return msg, nil, err
	}
	return llmChat(ctx, cfg, messages, tools)
}

func (s *Server) nativeLLMChat(ctx context.Context, cfg *config.LLMConfig, messages []chatMessage, tools []toolDef) (*chatMessage, error) {
	prompt, err := buildNativeLLMPrompt(messages, tools)
	if err != nil {
		return nil, err
	}

	var raw string
	switch cfg.Provider {
	case aiaccess.ProviderCodexCLI:
		raw, err = runCodexNativeLLM(ctx, cfg.Model, prompt)
	case aiaccess.ProviderClaudeCodeCLI:
		raw, err = runClaudeNativeLLM(ctx, cfg.Model, prompt)
	default:
		err = fmt.Errorf("unsupported native provider: %s", cfg.Provider)
	}
	if err != nil {
		return nil, err
	}

	parsed, err := parseNativeLLMResponse(raw)
	if err != nil {
		return nil, err
	}
	msg := &chatMessage{
		Role:    "assistant",
		Content: parsed.Content,
	}
	for i, tc := range parsed.ToolCalls {
		name := strings.TrimSpace(tc.Name)
		if name == "" {
			continue
		}
		args, err := nativeToolArgumentsJSON(tc.Arguments)
		if err != nil {
			return nil, err
		}
		msg.ToolCalls = append(msg.ToolCalls, toolCall{
			ID:   fmt.Sprintf("native:%s:%d", cfg.Provider, i),
			Type: "function",
			Function: functionCall{
				Name:      name,
				Arguments: args,
			},
		})
	}
	return msg, nil
}

func buildNativeLLMPrompt(messages []chatMessage, tools []toolDef) (string, error) {
	toolData, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal native tools: %w", err)
	}
	messageData, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal native messages: %w", err)
	}

	return fmt.Sprintf(`You are the model backend for PassGo Web's VM chat assistant.

Return ONLY a JSON object matching this schema:
{
  "content": "User-visible assistant text. Include a short explanation before tool calls.",
  "tool_calls": [
    {
      "name": "one of the available PassGo tool names",
      "arguments": "{\"name\":\"vm-name\"}"
    }
  ]
}

Use tool_calls only when the user explicitly asks PassGo to perform an action or fetch information not already in the current state.
If no tool is needed, return "tool_calls": [].
Do not call shell commands, edit files, or use native CLI tools. PassGo will execute only the JSON tool calls returned here.
Do not invent tools. The arguments field MUST be a JSON-encoded object string matching the selected tool's parameters.
For an empty-argument tool, use "arguments": "{}".

Available PassGo tools:
%s

Conversation messages:
%s
`, string(toolData), string(messageData)), nil
}

func runCodexNativeLLM(ctx context.Context, model string, prompt string) (string, error) {
	path, err := nativeLLMLookPath("codex")
	if err != nil {
		return "", fmt.Errorf("Codex CLI is not installed or not on PATH")
	}

	schemaPath, cleanupSchema, err := writeNativeLLMSchemaTempFile()
	if err != nil {
		return "", err
	}
	defer cleanupSchema()

	outputFile, err := os.CreateTemp("", "passgo-codex-output-*.txt")
	if err != nil {
		return "", fmt.Errorf("create Codex output file: %w", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"--cd", nativeWorkDir(),
		"--sandbox", "read-only",
		"--output-schema", schemaPath,
		"--output-last-message", outputPath,
		"--color", "never",
	}
	if model = strings.TrimSpace(model); model != "" && model != "codex-default" {
		args = append(args, "--model", model)
	}
	args = append(args, "-")

	runCtx, cancel := context.WithTimeout(ctx, nativeLLMTimeout)
	defer cancel()
	stdout, stderr, err := nativeLLMRunCommand(runCtx, path, args, prompt)
	if err != nil {
		return "", nativeCommandError("Codex CLI", runCtx, stdout, stderr, err)
	}

	if data, readErr := os.ReadFile(outputPath); readErr == nil && len(bytes.TrimSpace(data)) > 0 {
		return string(data), nil
	}
	return stdout, nil
}

func runClaudeNativeLLM(ctx context.Context, model string, prompt string) (string, error) {
	path, err := nativeLLMLookPath("claude")
	if err != nil {
		return "", fmt.Errorf("Claude Code CLI is not installed or not on PATH")
	}

	args := []string{
		"--print",
		"--output-format", "json",
		"--input-format", "text",
		"--tools", "",
		"--permission-mode", "dontAsk",
		"--no-session-persistence",
		"--json-schema", nativeLLMSchemaJSON(),
	}
	if model = strings.TrimSpace(model); model != "" && model != "claude-default" {
		args = append(args, "--model", model)
	}

	runCtx, cancel := context.WithTimeout(ctx, nativeLLMTimeout)
	defer cancel()
	stdout, stderr, err := nativeLLMRunCommand(runCtx, path, args, prompt)
	if err != nil {
		return "", nativeCommandError("Claude Code CLI", runCtx, stdout, stderr, err)
	}
	return stdout, nil
}

func runNativeLLMCommand(ctx context.Context, path string, args []string, stdin string) (string, string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = nativeWorkDir()
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func nativeCommandError(label string, ctx context.Context, stdout, stderr string, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%s timed out", label)
	}
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = strings.TrimSpace(stdout)
	}
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("%s failed: %s", label, truncate(detail, 500))
}

func parseNativeLLMResponse(raw string) (*nativeLLMResponse, error) {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return nil, fmt.Errorf("native CLI returned an empty response")
	}

	if resp, ok := parseNativeLLMObject(payload); ok {
		return resp, nil
	}

	var wrapper map[string]any
	if err := json.Unmarshal([]byte(payload), &wrapper); err == nil {
		for _, key := range []string{"result", "content", "message"} {
			if nested, ok := wrapper[key].(string); ok {
				if resp, nestedOK := parseNativeLLMObject(strings.TrimSpace(nested)); nestedOK {
					return resp, nil
				}
			}
		}
	}

	start := strings.Index(payload, "{")
	end := strings.LastIndex(payload, "}")
	if start >= 0 && end > start {
		if resp, ok := parseNativeLLMObject(payload[start : end+1]); ok {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("native CLI returned invalid structured response")
}

func parseNativeLLMObject(payload string) (*nativeLLMResponse, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		return nil, false
	}
	if _, ok := fields["content"]; !ok {
		return nil, false
	}
	if _, ok := fields["tool_calls"]; !ok {
		return nil, false
	}

	var resp nativeLLMResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		return nil, false
	}
	if resp.ToolCalls == nil {
		resp.ToolCalls = []nativeLLMToolCall{}
	}
	return &resp, true
}

func nativeToolArgumentsJSON(raw json.RawMessage) (string, error) {
	args := bytes.TrimSpace(raw)
	if len(args) == 0 || bytes.Equal(args, []byte("null")) {
		return "{}", nil
	}
	if len(args) > 0 && args[0] == '"' {
		var encoded string
		if err := json.Unmarshal(args, &encoded); err != nil {
			return "", fmt.Errorf("parse native tool arguments string: %w", err)
		}
		args = []byte(strings.TrimSpace(encoded))
	}
	if len(args) == 0 {
		return "{}", nil
	}
	if !json.Valid(args) {
		return "", fmt.Errorf("native tool arguments are not valid JSON")
	}
	var obj map[string]any
	if err := json.Unmarshal(args, &obj); err != nil {
		return "", fmt.Errorf("native tool arguments must be a JSON object: %w", err)
	}
	return string(args), nil
}

func writeNativeLLMSchemaTempFile() (string, func(), error) {
	f, err := os.CreateTemp("", "passgo-native-schema-*.json")
	if err != nil {
		return "", nil, fmt.Errorf("create native schema file: %w", err)
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := io.WriteString(f, nativeLLMSchemaJSON()); err != nil {
		f.Close()
		cleanup()
		return "", nil, fmt.Errorf("write native schema file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close native schema file: %w", err)
	}
	return path, cleanup, nil
}

func nativeLLMSchemaJSON() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "required": ["content", "tool_calls"],
  "properties": {
    "content": {
      "type": "string"
    },
    "tool_calls": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["name", "arguments"],
        "properties": {
          "name": {
            "type": "string"
          },
          "arguments": {
            "type": "string",
            "description": "JSON-encoded object string for the tool arguments, such as \"{}\" or \"{\\\"name\\\":\\\"vm1\\\"}\""
          }
        }
      }
    }
  }
}`
}

func nativeProviderReady(ctx context.Context, provider string) error {
	var command nativeAuthCommand
	switch provider {
	case aiaccess.ProviderCodexCLI:
		command = nativeAuthCommands[0]
	case aiaccess.ProviderClaudeCodeCLI:
		command = nativeAuthCommands[1]
	default:
		return fmt.Errorf("unsupported native provider: %s", provider)
	}
	status := checkNativeAuthProvider(ctx, command)
	if !status.Installed {
		return fmt.Errorf("%s is not installed or not on PATH", command.label)
	}
	if !status.Authenticated {
		return fmt.Errorf("%s is not signed in for the server user", command.label)
	}
	return nil
}

func nativeProviderModels(provider string) []aiaccess.Model {
	switch provider {
	case aiaccess.ProviderCodexCLI:
		return []aiaccess.Model{
			{ID: "codex-default", Name: "Codex CLI default"},
			{ID: "gpt-5.5", Name: "GPT 5.5"},
		}
	case aiaccess.ProviderClaudeCodeCLI:
		return []aiaccess.Model{
			{ID: "claude-default", Name: "Claude Code default"},
			{ID: "sonnet", Name: "Claude Sonnet"},
			{ID: "opus", Name: "Claude Opus"},
		}
	default:
		return []aiaccess.Model{}
	}
}

func nativeProviderLabel(provider string) string {
	switch provider {
	case aiaccess.ProviderCodexCLI:
		return "ChatGPT (Codex CLI)"
	case aiaccess.ProviderClaudeCodeCLI:
		return "Claude Code"
	default:
		return provider
	}
}

func nativeWorkDir() string {
	return filepath.Clean(os.TempDir())
}
