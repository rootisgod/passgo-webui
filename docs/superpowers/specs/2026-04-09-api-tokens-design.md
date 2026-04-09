# API Tokens & API Guide

## Context

PassGo Web has a complete REST API (60+ endpoints) but authentication is session-cookie-only. External tools (scripts, CI/CD, Home Assistant, etc.) can't use the API without a browser login. Adding persistent API tokens unlocks the full value of the existing API surface for automation.

## Data Model

### APIToken struct (config.go)

```go
type APIToken struct {
    ID        string `json:"id"`         // UUID
    Name      string `json:"name"`       // User-given label, e.g. "Home Assistant"
    Prefix    string `json:"prefix"`     // First 8 chars of token for display
    Hash      string `json:"hash"`       // SHA-256 hex of the full token
    CreatedAt string `json:"created_at"` // RFC3339 timestamp
}
```

Added to `Config` as `APITokens []APIToken json:"api_tokens,omitempty"`. Persisted in `~/.passgo-web/config.json`.

### Token format

`pgo_` + 32 random hex bytes = 68 characters total. Example: `pgo_a1b2c3d4e5f6...`

The `pgo_` prefix makes tokens grep-able and distinguishes them from session tokens.

### Storage

- Only the SHA-256 hash is stored in config.json, never the raw token.
- The raw token is returned once at creation time and never retrievable again.
- Config export/import excludes `api_tokens` (same treatment as `password`).

## Auth Flow

The existing auth middleware (`middleware.go:48-74`) already checks Bearer tokens against the session store. Add a third check after sessions:

```
Request → session cookie? → Bearer in session store? → Bearer matches API token hash? → 401
```

To check API tokens: SHA-256 hash the incoming Bearer value, compare against stored hashes. This is a linear scan over the token list — fine for the expected scale (handful of tokens).

API token requests:
- Skip login rate limiting (pre-authenticated)
- Still go through API rate limiting, body size limits, CORS, security headers
- Are authenticated for all protected endpoints (same access as a logged-in session)

## API Endpoints

All protected by existing auth middleware.

```
GET    /api/v1/tokens       → list tokens [{id, name, prefix, created_at}] (never returns hash)
POST   /api/v1/tokens       → create token {name: string} → {id, name, prefix, token} (token shown ONCE)
DELETE /api/v1/tokens/{id}   → revoke token → 200
```

### Validation

- `POST /tokens`: name required, max 64 chars, no duplicates
- `DELETE /tokens/{id}`: 404 if not found

### Backend files

- `internal/config/config.go`: `APIToken` struct, `AddAPIToken()`, `DeleteAPIToken()`, `GetAPITokens()` methods on Config
- `internal/api/handlers_tokens.go`: new handler file for the three endpoints
- `internal/api/routes.go`: register endpoints, wire token validation into auth middleware
- `internal/api/middleware.go`: add API token hash check after session check
- `internal/api/handlers_configbundle.go`: exclude `api_tokens` from export

## Frontend

### Sidebar

New `__api-tokens__` node in TreeSidebar.vue, between Schedules and Settings. Uses `KeyRound` icon from lucide-vue-next.

### ApiTokensPanel.vue

Two-tab layout matching existing panel patterns:

**Tab 1: Tokens**

- "Create Token" button opens inline form: single text input for token name + Create button
- On creation: shows the full token value in a highlighted box with:
  - Copy-to-clipboard button
  - Warning: "Copy this token now. It won't be shown again."
  - Dismiss button to clear the display
- Token list table: Name | Token Prefix (`pgo_a1b2...`) | Created | Delete button
- Empty state: "No API tokens yet. Create one to use the REST API from scripts and external tools."
- Delete: confirmation via ConfirmModal

**Tab 2: API Guide**

Static content rendered from a Vue component. Sections:

1. **Authentication** — How to use Bearer tokens with curl and fetch examples
2. **Quick Examples** — curl commands for common operations:
   - List all VMs
   - Get VM details
   - Start/stop a VM
   - Create a VM
   - List snapshots
   - Execute a command in a VM
3. **Endpoint Reference** — table grouped by category:
   - VMs (CRUD, lifecycle, exec, config)
   - Snapshots
   - Mounts & File Transfer
   - Cloud-Init Templates
   - Groups
   - Profiles
   - Schedules
   - Ansible
   - System (host resources, networks, images, version)
   - Chat/LLM

   Each row: Method | Path | Description

4. **Coming Soon** — note about Postman collection download

All curl examples use `YOUR_TOKEN_HERE` placeholder and `localhost:8080` as the default host.

### Frontend files

- `frontend/src/components/settings/ApiTokensPanel.vue`: main panel with tabs
- `frontend/src/api/client.js`: add `getTokens()`, `createToken(name)`, `deleteToken(id)` functions
- `frontend/src/components/layout/TreeSidebar.vue`: add sidebar node
- `frontend/src/App.vue`: add panel routing for `__api-tokens__`

## Verification

1. **Create token**: Log in, navigate to API Tokens, create a token named "test". Verify the full token is displayed once.
2. **Use token**: Copy token, use `curl -H "Authorization: Bearer pgo_..." http://localhost:8080/api/v1/vms` — should return VM list.
3. **Token persistence**: Restart the server, repeat the curl — token should still work.
4. **Revoke token**: Delete the token in the UI, repeat curl — should get 401.
5. **List tokens**: Verify the list shows name, prefix, and created date but never the full token or hash.
6. **Config export**: Export config, verify `api_tokens` is not included in the bundle.
7. **API Guide tab**: Verify all sections render, curl examples are correct, endpoint table covers all routes.
