package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

// OpenAI-compatible message types

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type llmChatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []toolDef     `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
}

type llmChatResponse struct {
	Choices []llmChoice `json:"choices"`
}

type llmChoice struct {
	Message      chatMessage `json:"message"`
	Delta        chatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type streamEvent struct {
	Type    string // "token", "done"
	Content string
}

var llmHTTPClient = &http.Client{Timeout: 120 * time.Second}

// llmChat makes a non-streaming call to the OpenAI-compatible chat completions endpoint.
// Used during the tool-calling loop where we need the full response before proceeding.
func llmChat(ctx context.Context, cfg *config.LLMConfig, messages []chatMessage, tools []toolDef) (*chatMessage, error) {
	reqBody := llmChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var chatResp llmChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	msg := chatResp.Choices[0].Message
	return &msg, nil
}

// llmChatStream makes a streaming call and sends tokens to the returned channel.
// Used for the final text response to provide real-time output.
func llmChatStream(ctx context.Context, cfg *config.LLMConfig, messages []chatMessage) (<-chan streamEvent, error) {
	reqBody := llmChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   true,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return nil, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	ch := make(chan streamEvent, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				ch <- streamEvent{Type: "done"}
				return
			}

			var chunk llmChatResponse
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				ch <- streamEvent{Type: "token", Content: delta.Content}
			}
		}
		ch <- streamEvent{Type: "done"}
	}()

	return ch, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
