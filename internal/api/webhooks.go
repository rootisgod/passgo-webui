package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

// WebhookDispatcher is called after every event emission to fire matching webhooks.
type WebhookDispatcher interface {
	DispatchWebhooks(event Event)
}

type webhookPayload struct {
	Event   Event          `json:"event"`
	Webhook webhookPayloadMeta `json:"webhook"`
}

type webhookPayloadMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DispatchWebhooks sends HTTP POST requests to all matching enabled webhooks.
func (s *Server) DispatchWebhooks(event Event) {
	// Prevent infinite loops: don't fire webhooks for webhook delivery events
	if event.Category == "webhook" {
		return
	}

	s.groupMu.Lock()
	webhooks := make([]config.Webhook, len(s.cfg.Webhooks))
	copy(webhooks, s.cfg.Webhooks)
	s.groupMu.Unlock()

	for _, wh := range webhooks {
		if !wh.Enabled {
			continue
		}
		if !webhookMatchesEvent(wh, event) {
			continue
		}
		go s.fireWebhook(wh, event)
	}
}

func webhookMatchesEvent(wh config.Webhook, ev Event) bool {
	if len(wh.Categories) > 0 && !contains(wh.Categories, ev.Category) {
		return false
	}
	if len(wh.Results) > 0 && !contains(wh.Results, ev.Result) {
		return false
	}
	return true
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func (s *Server) fireWebhook(wh config.Webhook, event Event) {
	payload := webhookPayload{
		Event: event,
		Webhook: webhookPayloadMeta{
			ID:   wh.ID,
			Name: wh.Name,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		s.eventLog.EmitEvent("webhook", "deliver", "system", wh.Name, "failed", "marshal error: "+err.Error())
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
	if err != nil {
		s.eventLog.EmitEvent("webhook", "deliver", "system", wh.Name, "failed", "request error: "+err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PassGo-Web/1.0")

	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-PassGo-Signature", sig)
	}

	resp, err := client.Do(req)
	if err != nil {
		s.eventLog.EmitEvent("webhook", "deliver", "system", wh.Name, "failed", err.Error())
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.eventLog.EmitEvent("webhook", "deliver", "system", wh.Name, "success", resp.Status)
	} else {
		s.eventLog.EmitEvent("webhook", "deliver", "system", wh.Name, "failed", resp.Status)
	}
}
