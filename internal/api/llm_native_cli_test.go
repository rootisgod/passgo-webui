package api

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/rootisgod/passgo-web/internal/aiaccess"
	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func TestNativeLLMChatParsesToolCalls(t *testing.T) {
	restore := stubNativeLLM(
		func(binary string) (string, error) {
			if binary != "codex" {
				t.Fatalf("binary = %q, want codex", binary)
			}
			return "/usr/local/bin/codex", nil
		},
		func(ctx context.Context, path string, args []string, stdin string) (string, string, error) {
			if !strings.Contains(stdin, "Available PassGo tools") {
				t.Fatal("prompt did not include PassGo tool definitions")
			}
			writeCodexOutputFile(t, args, `{"content":"Listing VMs.","tool_calls":[{"name":"list_vms","arguments":"{}"}]}`)
			return "", "", nil
		},
	)
	defer restore()

	s := &Server{}
	msg, err := s.nativeLLMChat(context.Background(), &config.LLMConfig{Provider: aiaccess.ProviderCodexCLI}, []chatMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "list vms"},
	}, filterToolsForMode(false))
	if err != nil {
		t.Fatalf("nativeLLMChat: %v", err)
	}
	if msg.Content != "Listing VMs." {
		t.Fatalf("content = %q", msg.Content)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "list_vms" || msg.ToolCalls[0].Function.Arguments != "{}" {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}
}

func TestRunAgentLoopExecutesNativeToolCalls(t *testing.T) {
	const listJSON = `{"list":[{"name":"vm1","state":"Running","ipv4":["10.0.0.2"],"release":"Ubuntu 24.04 LTS"}]}`
	call := 0
	restore := stubNativeLLM(
		func(string) (string, error) {
			return "/usr/local/bin/codex", nil
		},
		func(ctx context.Context, path string, args []string, stdin string) (string, string, error) {
			call++
			if call == 1 {
				writeCodexOutputFile(t, args, `{"content":"I'll list the VMs.\n","tool_calls":[{"name":"list_vms","arguments":"{}"}]}`)
			} else {
				writeCodexOutputFile(t, args, `{"content":"vm1 is running at 10.0.0.2.","tool_calls":[]}`)
			}
			return "", "", nil
		},
	)
	defer restore()

	runner := func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "info --all --format json":
			return "", os.ErrNotExist
		case "list --format json":
			return listJSON, nil
		default:
			t.Fatalf("unexpected multipass call: %v", args)
			return "", nil
		}
	}
	s := &Server{
		mp:     multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg:    &config.Config{LLM: &config.LLMConfig{Provider: aiaccess.ProviderCodexCLI}, Groups: []string{}, VMGroups: map[string]string{}},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	eventCh := make(chan sseEvent, 64)
	s.runAgentLoop(context.Background(), s.cfg.LLM, []chatMessage{{Role: "user", Content: "list vms"}}, map[string]bool{}, eventCh)

	var sawTool bool
	var final string
	for ev := range eventCh {
		if ev.Type == "tool_done" && ev.Name == "list_vms" {
			sawTool = true
		}
		if ev.Type == "token" {
			final += ev.Content
		}
	}
	if !sawTool {
		t.Fatal("native tool call was not executed")
	}
	if !strings.Contains(final, "vm1 is running") {
		t.Fatalf("final response = %q", final)
	}
}

func TestParseNativeLLMResponseFromClaudeWrapper(t *testing.T) {
	resp, err := parseNativeLLMResponse(`{"type":"result","result":"{\"content\":\"ok\",\"tool_calls\":[]}"}`)
	if err != nil {
		t.Fatalf("parseNativeLLMResponse: %v", err)
	}
	if resp.Content != "ok" || len(resp.ToolCalls) != 0 {
		t.Fatalf("response = %+v", resp)
	}
}

func stubNativeLLM(
	lookPath func(string) (string, error),
	runCommand func(context.Context, string, []string, string) (string, string, error),
) func() {
	oldLookPath := nativeLLMLookPath
	oldRunCommand := nativeLLMRunCommand
	nativeLLMLookPath = lookPath
	nativeLLMRunCommand = runCommand
	return func() {
		nativeLLMLookPath = oldLookPath
		nativeLLMRunCommand = oldRunCommand
	}
}

func writeCodexOutputFile(t *testing.T, args []string, content string) {
	t.Helper()
	for i, arg := range args {
		if arg == "--output-last-message" && i+1 < len(args) {
			if err := os.WriteFile(args[i+1], []byte(content), 0600); err != nil {
				t.Fatalf("write Codex output: %v", err)
			}
			return
		}
	}
	t.Fatal("Codex args did not include --output-last-message")
}
