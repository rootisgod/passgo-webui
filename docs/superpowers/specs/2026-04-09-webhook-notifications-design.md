# Webhook Notifications Design

## Context

PassGo Web already has a comprehensive event log (`eventlog.go`) that captures every state-changing action across VMs, schedules, Ansible runs, LLM chat, and config changes. Webhook notifications build on this — when an event is emitted, matching webhooks fire HTTP POSTs to user-configured URLs. This integrates PassGo with Slack, Discord, ntfy, Home Assistant, or any HTTP endpoint.

## Data Model

### Webhook struct (config.go)

```go
type Webhook struct {
    ID         string   `json:"id"`
    Name       string   `json:"name"`
    URL        string   `json:"url"`
    Enabled    bool     `json:"enabled"`
    Categories []string `json:"categories"`  // empty = all
    Results    []string `json:"results"`     // empty = all
    Secret     string   `json:"secret"`      // optional HMAC-SHA256 key
    CreatedAt  string   `json:"created_at"`
}
```

Stored in `config.json` as `"webhooks": [...]` on the Config struct. CRUD helpers follow the Schedule/APIToken pattern: `AddWebhook`, `UpdateWebhook`, `DeleteWebhook`, `GetWebhook`. Protected by `groupMu`.

### Filter semantics

- **Categories**: `vm`, `schedule`, `ansible`, `llm`, `config`. Empty array matches all.
- **Results**: `success`, `failed`, `partial`, `no_targets`. Empty array matches all.
- An event matches a webhook if: `(categories is empty OR event.Category in categories) AND (results is empty OR event.Result in results)`.

## Dispatch Flow

### Integration point: EventLog.Emit()

After writing the event to JSONL and updating the in-memory cache, `Emit()` calls a `WebhookDispatcher` interface (or callback) injected at construction time. This keeps EventLog decoupled from Server.

```go
type WebhookDispatcher interface {
    DispatchWebhooks(event Event)
}
```

### Server.DispatchWebhooks(event Event)

1. Skip if `event.Category == "webhook"` (loop prevention)
2. Load enabled webhooks from config
3. For each matching webhook, launch a goroutine:
   a. Marshal payload JSON
   b. If `secret` is set, compute `HMAC-SHA256(secret, payload)` and set `X-PassGo-Signature` header
   c. POST with 10-second timeout, `Content-Type: application/json`
   d. Emit a delivery event: category=`webhook`, action=`deliver`, resource=webhook name, result=`success`/`failed`, detail=status code or error

### Payload format

```json
{
  "event": {
    "id": "1712649600-abc123",
    "timestamp": "2026-04-09T10:00:00Z",
    "category": "schedule",
    "action": "stop",
    "actor": "scheduler",
    "resource": "nightly-shutdown",
    "result": "failed",
    "detail": "vm1: instance does not exist"
  },
  "webhook": {
    "id": "wh_a1b2c3",
    "name": "Notify ntfy on failures"
  }
}
```

### Loop prevention

Events with `category == "webhook"` are never dispatched to webhooks. This prevents webhook delivery logs from triggering further webhook calls.

## API Endpoints

```
GET    /webhooks            — list all webhooks
POST   /webhooks            — create webhook (returns created webhook with id)
PUT    /webhooks/{id}       — update webhook
DELETE /webhooks/{id}       — delete webhook
POST   /webhooks/{id}/test  — send a synthetic test event
```

All protected by auth middleware. Handlers in `handlers_webhooks.go`, following the schedules/tokens handler pattern.

### Test endpoint

Sends a synthetic event through the dispatch flow for the specified webhook:
```json
{
  "category": "webhook",
  "action": "test",
  "actor": "user",
  "resource": "test",
  "result": "success",
  "detail": "This is a test notification from PassGo Web"
}
```
The test endpoint directly POSTs to the webhook URL rather than going through the normal dispatch flow, so loop prevention doesn't apply.

## Config Export/Import

- Export: include webhooks but **exclude the `secret` field** (same pattern as API tokens excluded, API key stripped)
- Import: merge webhooks array, preserve existing secrets if webhook IDs match

## Frontend

### WebhooksPanel.vue

- New sidebar node "Webhooks" with Bell icon from lucide-vue-next
- Placed after "Schedules" in the tree sidebar
- Follows SchedulesPanel pattern: list view with inline edit forms

### List view (per webhook row)

- Name, URL (truncated with ellipsis), category badges (colored like EventLogPanel), enabled toggle
- Action buttons: Test, Edit, Delete

### Edit/Create form

- Name (text input, required)
- URL (text input, required, validated as URL)
- Categories (checkboxes: VM, Schedule, Ansible, LLM, Config — unchecked = all)
- Results (checkboxes: Success, Failed, Partial — unchecked = all)
- Secret (password-masked text input, optional)
- Enabled (toggle)

### API client additions (api/client.js)

```js
export const listWebhooks = () => request('GET', '/webhooks')
export const createWebhook = (webhook) => request('POST', '/webhooks', webhook)
export const updateWebhook = (id, webhook) => request('PUT', `/webhooks/${encodeURIComponent(id)}`, webhook)
export const deleteWebhook = (id) => request('DELETE', `/webhooks/${encodeURIComponent(id)}`)
export const testWebhook = (id) => request('POST', `/webhooks/${encodeURIComponent(id)}/test`)
```

## Files to Create/Modify

### New files
- `internal/api/webhooks.go` — dispatch logic, WebhookDispatcher interface
- `internal/api/handlers_webhooks.go` — CRUD + test HTTP handlers
- `frontend/src/components/WebhooksPanel.vue` — UI panel

### Modified files
- `internal/config/config.go` — Webhook struct, Config.Webhooks field, CRUD helpers
- `internal/api/eventlog.go` — Accept and call WebhookDispatcher after Emit
- `internal/api/server.go` — Register webhook routes, wire dispatcher into EventLog
- `internal/api/handlers_configbundle.go` — Include webhooks in export/import
- `frontend/src/api/client.js` — Webhook API methods
- `frontend/src/components/TreeSidebar.vue` — Add Webhooks node
- `frontend/src/App.vue` — Render WebhooksPanel when selected

## Verification

1. **Unit**: Test filter matching logic (category/result combinations, empty = match all)
2. **Integration**: Create a webhook via API, trigger an event, verify HTTP POST received (use httptest server)
3. **Manual**: Create webhook pointing to https://ntfy.sh/test-topic, stop a VM, verify notification arrives
4. **Loop prevention**: Verify webhook delivery events don't trigger further webhooks
5. **Test endpoint**: Create webhook, hit /test, verify payload arrives
6. **Config export/import**: Export with webhooks, verify secret is excluded, import and verify webhooks restored
7. **UI**: Create/edit/delete webhooks, test button shows toast, enable/disable toggle works
