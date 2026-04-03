<!-- GSD:project-start source:PROJECT.md -->
## Project

**PassGo Web**

A web-based management interface for Canonical's Multipass, modelled on the Proxmox/vSphere UI pattern. Runs on the same machine as Multipass and exposes both a browser UI and a REST API for managing virtual machine instances. A homelab tool — functional, stable, and simple.

**Core Value:** A clean, familiar tree-based web interface that covers 90%+ of what Multipass can do, with an API complete enough for external automation.

### Constraints

- **Tech stack:** Go 1.22+ backend, Vue 3 + Vite + Tailwind CSS v3 frontend, no additional UI component library
- **Dependencies (backend):** `github.com/creack/pty` for PTY, `github.com/gorilla/websocket` for WebSocket, optionally `github.com/go-chi/chi/v5` if routing gets complex
- **Dependencies (frontend):** Vue 3, Pinia, xterm.js + addons, lucide-vue-next, @tailwindcss/forms, CodeMirror 6 (@codemirror/lang-yaml, @codemirror/lint, @codemirror/view, @codemirror/state), js-yaml
- **Binary:** Single executable with embedded frontend via `//go:embed frontend/dist/*`
- **Config:** `~/.passgo-web/config.json` with bcrypt-hashed password
- **No Vue Router:** Single-page app driven by tree selection, not URL routing
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Spec Validation Summary
## Recommended Stack
### Backend -- Go
| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.24+ (recommend 1.26.1) | Language runtime | Latest stable is 1.26.1 (Mar 2026). 1.22+ routing patterns available in all supported versions. Use latest for security patches. | HIGH |
| net/http (stdlib) | -- | HTTP server + routing | Go 1.22+ ServeMux supports method matching (`GET /api/vms`) and path wildcards (`/api/vms/{name}`). Eliminates need for chi in most cases. Chi is a fine fallback if middleware composition gets complex. | HIGH |
| github.com/coder/websocket | latest (v1.8+) | WebSocket connections | Actively maintained successor to nhooyr/websocket. Context-aware, safe concurrent writes, no archived-dependency risk. gorilla/websocket is archived since late 2022 -- do not use for new projects. | HIGH |
| github.com/creack/pty | v1.1.24 | PTY allocation for shell | The standard Go PTY library. Latest release adds z/OS support. No real alternatives in the Go ecosystem. Stable and well-tested. | HIGH |
| github.com/go-chi/chi/v5 | v5.2.3 | Router (fallback only) | Only pull in if net/http ServeMux becomes insufficient -- e.g., complex middleware chains, route grouping with shared middleware. For this project's REST API, stdlib should be enough. | MEDIUM |
| log/slog (stdlib) | -- | Structured logging | Standard since Go 1.21. JSON output for structured logs, text for dev. No need for zerolog/zap. | HIGH |
| crypto/bcrypt (golang.org/x/crypto) | latest | Password hashing | Required for the single-user auth. Part of extended stdlib. | HIGH |
| embed (stdlib) | -- | Frontend asset embedding | `//go:embed all:frontend/dist` bundles the built Vue app into the binary. Use `fs.Sub` to strip the path prefix. Use build tags for dev mode (serve from disk) vs production (serve from embed). | HIGH |
### Frontend -- Vue 3
| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Vue | 3.5.31 | UI framework | Current stable. 3.6 is in beta -- do not use for a new project launch. Composition API + `<script setup>` is the standard pattern. | HIGH |
| Vite | 6.x (LTS) or 8.0.3 | Build tool + dev server | Vite 8 (Mar 2026) uses Rolldown for 10-30x faster builds. However, Vite 6 is the current LTS and more battle-tested. For a project this size, build speed is not a bottleneck -- Vite 6 LTS is the safer choice. If you want bleeding edge, Vite 8 works but has been stable for only ~3 weeks. | MEDIUM |
| Pinia | 3.x | State management | Official Vue state management. v3 dropped Vue 2 support, cleaner API. Composition-style stores with `defineStore`. | HIGH |
| Tailwind CSS | 4.x | Utility CSS | v4 stable since Jan 2025. Rust-based Oxide engine: 5-10x faster builds. CSS-first config via `@theme` directives replaces tailwind.config.js. No reason to start on v3 in 2026. | HIGH |
| @xterm/xterm | 6.0.0 | Terminal emulator | Latest major version. 30% smaller bundle than v5. Use scoped @xterm/* packages -- the old `xterm` package is deprecated. Required addons: @xterm/addon-fit, @xterm/addon-webgl (for GPU rendering). | HIGH |
| @lucide/vue | 1.x | Icons | Renamed from lucide-vue-next in v1.0. Tree-shakeable, consistent design. Good fit for infrastructure UIs. | HIGH |
| @tailwindcss/forms | latest | Form reset styles | Compatible with Tailwind v4 via `@plugin "@tailwindcss/forms"` directive in CSS. Provides sensible form element defaults. | HIGH |
### Infrastructure
| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| go:embed | stdlib | Single binary packaging | Embeds `frontend/dist/*` into Go binary at compile time. Standard approach for Go+SPA single binaries. | HIGH |
| Makefile or Task | -- | Build orchestration | Need to: (1) build frontend with Vite, (2) embed into Go binary, (3) cross-compile. A Makefile with `build-frontend` and `build-backend` targets is the simplest approach. | HIGH |
| goreleaser | latest | Cross-platform releases | Handles cross-compilation for Windows/macOS/Linux with proper naming and checksums. Optional but saves time vs manual Makefile cross-compile targets. | MEDIUM |
### Supporting Libraries
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/go-chi/chi/v5 | v5.2.3 | Middleware/routing | Only if stdlib routing proves insufficient for middleware grouping |
| encoding/json (stdlib) | -- | JSON serialization | Parsing multipass `--format json` output, API request/response bodies |
| os/exec (stdlib) | -- | CLI execution | Running `multipass` commands. Pair with context for timeouts. |
| github.com/google/uuid | latest | Unique identifiers | Session tokens, request IDs if needed |
## Alternatives Considered
| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| WebSocket | coder/websocket | gorilla/websocket | Archived since 2022. Works but receives no security patches. Do not start new projects on archived deps. |
| WebSocket | coder/websocket | golang.org/x/net/websocket | Low-level, missing features (compression, ping/pong), not recommended even by Go team. |
| Router | net/http (stdlib) | go-chi/chi v5 | Chi adds middleware composition but stdlib covers this project's needs. Keep as fallback. |
| Router | net/http (stdlib) | gin, echo, fiber | Heavier frameworks with their own paradigms. Overkill for a REST API wrapping a CLI. |
| CSS | Tailwind v4 | Tailwind v3 | v3 works but is the previous generation. No reason to start on it in 2026. |
| CSS | Tailwind v4 | DaisyUI / Headless UI | Project spec explicitly says no component library. Custom components with Tailwind utilities is the right call for a Proxmox-style UI. |
| Icons | @lucide/vue | heroicons | Both fine. Lucide has more icons (1500+), consistent stroke style, tree-shakeable. Already in spec. |
| Terminal | @xterm/xterm | custom canvas | xterm.js is the industry standard (VS Code, Theia, Hyper all use it). No reason to build custom. |
| State | Pinia 3 | Vuex | Vuex is legacy. Pinia is the official successor. |
| Build | Vite 6 LTS | Webpack | Webpack is slower and more complex. Vite is the Vue ecosystem standard. |
## What NOT to Use
| Technology | Why Not |
|------------|---------|
| gorilla/websocket | Archived. Use coder/websocket instead. |
| Tailwind CSS v3 | Previous generation. v4 is stable and faster. |
| `xterm` (unscoped npm package) | Deprecated. Use `@xterm/xterm`. |
| `lucide-vue-next` | Renamed to `@lucide/vue` in v1.0. Old name still works but will eventually stop receiving updates. |
| Vue Router | Correctly excluded in spec. This is a tree-driven SPA, not a route-driven one. Adding Vue Router would fight the UI pattern. |
| Vuex | Legacy. Pinia is the official replacement. |
| Gin / Echo / Fiber | Framework overhead for what is fundamentally a thin REST layer over a CLI. |
| SQLite / any database | Correctly excluded. All state comes from multipass CLI. Adding persistence would create state sync bugs. |
| TypeScript (for this project) | The spec does not mention it, and for a project this size with one developer, the velocity gain from plain JS + `<script setup>` outweighs type safety. If the project grows significantly, reconsider. This is a pragmatic call, not a universal recommendation. |
## Installation
### Backend
# Initialize Go module
# Dependencies
# Optional (only if stdlib routing proves insufficient)
# go get github.com/go-chi/chi/v5
### Frontend
# Create Vue project
# Core
# UI
# Terminal
# CSS (Tailwind v4 with Vite plugin)
### Tailwind v4 Setup (CSS-first, no config file)
## Version Pinning Strategy
- `"vue": "^3.5.0"` -- float within 3.5.x
- `"@xterm/xterm": "^6.0.0"` -- float within 6.x
- `"pinia": "^3.0.0"` -- float within 3.x
- `"tailwindcss": "^4.0.0"` -- float within 4.x
## Key Architecture Notes from Stack Choices
## Sources
- [Go 1.22 routing enhancements](https://go.dev/blog/routing-enhancements) -- Official Go blog
- [Go release history](https://go.dev/doc/devel/release) -- Go 1.24, 1.25, 1.26 release dates
- [coder/websocket](https://github.com/coder/websocket) -- Maintained WebSocket library
- [gorilla/websocket archived status](https://github.com/gorilla/websocket) -- Archived repo
- [creack/pty releases](https://github.com/creack/pty/releases) -- v1.1.24 latest
- [Tailwind CSS v4 upgrade guide](https://tailwindcss.com/docs/upgrade-guide) -- Migration from v3
- [Tailwind CSS v4 compatibility](https://tailwindcss.com/docs/compatibility) -- Browser support
- [@xterm/xterm releases](https://github.com/xtermjs/xterm.js/releases) -- v6.0.0
- [Lucide v1.0](https://lucide.dev/guide/version-1) -- Package rename
- [Pinia official docs](https://pinia.vuejs.org/) -- v3.0 for Vue 3
- [Vite releases](https://vite.dev/releases) -- v6 LTS, v8 stable
- [Vue.js releases](https://vuejs.org/about/releases) -- v3.5.31 current stable
- [go-chi/chi releases](https://github.com/go-chi/chi/releases) -- v5.2.3
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

### Backend (Go)
- All multipass CLI interaction goes through `pkg/multipass/Client` methods using `c.run()` (which wraps `exec.Command("multipass", ...)`)
- API handlers are grouped by domain: `handlers_vms.go`, `handlers_system.go`, `handlers_cloudinit.go`, `handlers_groups.go`, `handlers_shell.go`, `handlers_chat.go`
- Response helpers: `writeJSON(w, status, v)`, `writeError(w, status, msg)`, `writeMessage(w, msg)` in `responses.go`
- Path parameters via Go 1.22+ `r.PathValue("name")`
- File safety: all user-facing file operations validate with `sanitizeTemplateName()` (regex + filepath.Rel check)
- Long-running operations (VM launch, clone) run in goroutines with in-memory status tracking via launchTracker
- Persistent PTY sessions via `ptyStore` (keyed by `vmName:sessionID`): shell processes survive WebSocket disconnects, 64KB scrollback ring buffer replays on reconnect, 30-min TTL reaper cleans idle sessions
- Platform-specific PTY code split via build tags: `pty_store_unix.go` (creack/pty) and `pty_store_windows.go` (conpty)
- VM groups stored in `config.json` as `groups []string` (ordered names) + `vm_groups map[string]string` (VM→group), protected by `groupMu` mutex
- Embedded assets use `//go:embed` in `cmd/server/main.go` and are passed to `api.NewServer()`
- LLM chat agent loop in `llm_agent.go`: orchestrates non-streaming tool calls then streams final response via SSE. System prompt refreshed every iteration with live VM/group state. Write-tool iterations capped at 50; read-only tools unlimited.
- LLM client in `llm_client.go`: OpenAI-compatible HTTP client (works with OpenRouter, Ollama, any `/v1/chat/completions` endpoint). Non-streaming mode for tool loop, streaming for final response.
- LLM tool definitions in `llm_tools.go`: 19 tools mapping to `multipass.Client` methods + config group operations. Tools classified as `readOnlyTools` (list/info) and `destructiveTools` (delete/restore, require user confirmation).
- LLM tool executor in `llm_executor.go`: switch dispatch from tool name to client method. Group tools use `groupMu` mutex + `config.Save()`. Tool errors returned as JSON for LLM to explain, not Go errors.
- LLM config in `config.go`: `LLMConfig` struct with `base_url`, `api_key`, `model`, `read_only` fields, nested under `Config.LLM`

### Frontend (Vue 3)
- All components use `<script setup>` composition API
- State management via Pinia stores (`vmStore.js`, `toastStore.js`, `chatStore.js`)
- API calls centralized in `api/client.js` using a single `request()` helper
- Styling: Tailwind utility classes with CSS custom properties (`var(--bg-primary)`, etc.) defined in `assets/main.css`
- Modals use `<Teleport to="body">`
- CodeMirror components must render outside Vue `<Transition>` to avoid DOM conflicts
- Icons from `lucide-vue-next`, passed as raw components to ActionButton via `markRaw()`
- Polling via `usePolling` composable (3s interval, pauses when tab hidden)
- Metrics history buffered in `useMetricsHistory.js` (reactive store, recorded on each poll)
- Sparkline component (`Sparkline.vue`) uses SVG viewBox for responsive width
- Context menu (`ContextMenu.vue`) is reusable: positioned dropdown with click-outside/Escape close
- Multi-VM selection via `selectedVms` array in vmStore, bulk actions use `Promise.allSettled`
- Tabs that run `multipass exec` (Files) must guard against stopped VMs to prevent auto-start
- Console supports multiple tabs per VM: each tab is an independent PTY session via `ConsoleTerminal.vue`, managed by `VmConsoleTab.vue` as a tab container; all terminals stay mounted (using `invisible` class) so WebSocket connections persist when switching tabs
- VM groups in sidebar: collapsible folder nodes with state badges, group context menu (start/stop/delete all, rename, delete group), "Move to Group..." in VM context menu
- `useWebSocket.js` composable is a factory (not singleton) — each terminal instance creates its own
- LLM chat panel (`ChatPanel.vue`): right-side blade, slides in/out, SSE streaming via fetch ReadableStream. Chat history persisted to localStorage. Markdown rendering via `useMarkdown.js` composable (code blocks, inline code, bold, italic, lists — no external dependency).
- Chat store (`chatStore.js`): IMPORTANT — all message mutations during SSE streaming must go through `this.messages[idx]` (reactive proxy), never a local variable reference. Local refs bypass Vue reactivity and the UI won't update.
- Destructive tool calls (delete_vm, restore_snapshot, delete_snapshot) require user confirmation via `confirm_required` SSE event → frontend confirmation banner → re-send with `confirmed_tools` array.
- Chat settings modal with searchable model dropdown fetched from provider's `/models` endpoint. "Connect" button saves key + fetches models. Read-only mode toggle restricts LLM to informational tools only.
- Auto-refresh: `tool_done` events for state-changing tools trigger `vmStore.fetchVMs()` immediately
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

### API Endpoints
```
Auth (public):     POST /auth/login, /auth/logout, GET /version
VMs (protected):   GET/POST /vms, GET /vms/{name}, POST /vms/{name}/start|stop|suspend|recover|clone
                   DELETE /vms/{name}, POST /vms/start-all|stop-all|purge, POST /vms/{name}/exec
                   GET/PUT /vms/{name}/config (read/resize CPU, memory, disk)
                   GET /vms/{name}/cloud-init/status
Snapshots:         GET/POST /vms/{name}/snapshots, POST .../restore, DELETE .../{snap}
Mounts:            GET/POST/DELETE /vms/{name}/mounts
Cloud-Init:        GET /cloud-init/templates (list all, built-in + user)
                   GET/POST/PUT/DELETE /cloud-init/templates/{name} (CRUD)
Launches:          GET /launches, DELETE /launches/{name} (async launch tracking)
Host:              GET /host/resources (CPU count, total RAM)
Networks:          GET /networks
Shell sessions:    POST /vms/{name}/shell/sessions (create), GET .../sessions (list),
                   DELETE .../sessions/{sessionId} (delete), WS /vms/{name}/shell/{sessionId}
Groups:            GET /groups, POST /groups, PUT /groups/{name} (rename),
                   DELETE /groups/{name}, PUT /groups/assign, PUT /groups/reorder
Chat / LLM:       POST /chat (SSE streaming), GET/PUT /chat/config, GET /chat/models
```

### Frontend Component Tree
```
App.vue
├── LoginPage.vue
├── AppHeader.vue
├── TreeSidebar.vue (host node, Cloud-Init node, group folders, VM list, context menus, multi-select + bulk actions)
│   ├── ContextMenu.vue (reusable right-click menu)
│   ├── CloneVmModal.vue
│   ├── ConfirmModal.vue
│   ├── GroupNameModal.vue (create/rename group)
│   └── MoveToGroupModal.vue (assign VM to group)
├── CloudInitPanel.vue (outside Transition — CodeMirror conflict)
│   └── CloudInitEditor.vue (CodeMirror 6 + js-yaml linter + cloud-init key/type validation)
├── HostPanel.vue (dashboard cards, launch progress/failures)
│   └── CreateVmModal.vue
├── VmDetailPanel.vue (tabbed)
│   ├── VmSummaryTab.vue + CloudInitStatus.vue + Sparkline.vue (resource timeline graphs)
│   ├── VmConsoleTab.vue (multi-tab container: tab bar + N ConsoleTerminal instances)
│   │   └── ConsoleTerminal.vue (single xterm.js + WebSocket session, power-on guard)
│   ├── VmSnapshotsTab.vue (clone from snapshot support)
│   ├── VmMountsTab.vue
│   ├── VmTransferTab.vue (file browser, power-on guard)
│   └── VmConfigTab.vue
├── ChatPanel.vue (right-side blade, SSE streaming, confirmation banner)
│   ├── ChatMessage.vue (user/assistant bubbles, markdown rendering, tool status)
│   └── ChatSettingsModal.vue (provider presets, API key + Connect, model dropdown, read-only toggle)
├── StatusBar.vue
└── Toast.vue
```

### Async VM Launch Flow
1. POST /vms → returns 202 immediately, goroutine runs `multipass launch`
2. Launch tracker stores {name, status, error} in memory
3. Frontend polls GET /launches alongside GET /vms every 3 seconds
4. Tree shows spinner for launches not yet in VM list; HostPanel shows progress banner
5. VMs appearing in multipass list with unknown state + active launch → tagged as "Creating"
6. On completion: tracker clears entry; on failure: stores error, user can dismiss
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
