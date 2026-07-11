# PassGo Web — Complete Project Specification

## 1. Vision

A web-based management interface for Canonical's Multipass, modelled on the Proxmox / vSphere UI pattern. The application runs on the same machine as Multipass and exposes both a browser UI and a REST API for managing virtual machine instances.

This is a homelab tool — functional, stable, and simple. Not a commercial product. Security is basic (username/password). The priority is covering 90%+ of what Multipass can do through a clean, familiar tree-based interface.

---

## 2. Hard Requirements

1. **Single binary** — one executable that serves the web UI, REST API, and invokes the `multipass` CLI. No separate processes, no containers, no database.
2. **Multiplatform** — build for Windows (amd64), macOS (amd64, arm64), and Linux (amd64, arm64). Cross-compile with `GOOS`/`GOARCH`. Use UPX for Linux/Windows where applicable (not macOS arm64).
3. **Go backend + Vue 3 frontend** — all server code in Go. Frontend built with Vue 3 + Vite + Tailwind CSS, compiled to static assets and embedded into the binary via `//go:embed`.
4. **Tree-based navigation** — left sidebar showing a tree of the host node with VMs nested underneath (the Proxmox/vSphere pattern). Clicking a node shows its detail panel on the right.
5. **API-first design** — the web UI is a consumer of the REST API. Every action the UI performs goes through the API. This means the API is complete enough for external automation from remote machines.
6. **90% multipass coverage** — instance creation (quick + advanced with cloud-init), power management (start/stop/suspend/delete/recover), snapshots (create/list/restore/delete), mounts (add/remove/list), shell access, network listing, purge.
7. **Simple authentication** — username/password, configured at first run or via config file. A single shared credential is fine. No RBAC, no OAuth.

---

## 3. Reference Project: PassGo TUI

The PassGo repository (https://github.com/rootisgod/passgo) is an existing Go TUI for Multipass. It contains well-tested multipass CLI interaction code that should be extracted and reused. The repo should be available in a local folder alongside this spec.

### 3.1 Files to REUSE (extract into `pkg/multipass/`)

Strip the `appLogger` global from all files. Replace with a `*slog.Logger` passed into a `Client` struct. All functions become methods on that struct.

**`multipass.go` (485 lines) — CLI wrapper. The most important file.**

Every function that calls the `multipass` binary lives here. Reuse directly:

- `runMultipassCommand(args ...string)` — central executor. Captures stdout/stderr, logs command and errors.
- `ListVMs()` → `multipass list`
- `GetVMInfo(name)` → `multipass info <name>`
- `StartVM(name)`, `StopVM(name)`, `DeleteVM(name, purge)`, `RecoverVM(name)` — lifecycle ops
- `LaunchVM(name, release)` — quick create
- `LaunchVMAdvanced(name, release, cpus, memoryMB, diskGB, networkName)` — advanced create. networkName behaviour: `""` = NAT, `"bridged"` = `--bridged` flag, anything else = `--network <name>`
- `LaunchVMWithCloudInit(name, release, cpus, memoryMB, diskGB, cloudInitFile, networkName)` — advanced create with cloud-init file path
- `ListNetworks()` → `multipass networks --format json`, returns `[]NetworkInfo`
- `CreateSnapshot(vmName, snapshotName, description)` → `multipass snapshot --name <n> --comment <c> <vm>`
- `ListSnapshots()` → `multipass list --snapshots`
- `RestoreSnapshot(vmName, snapshotName)` → `multipass restore --destructive <vm>.<snap>`
- `DeleteSnapshot(vmName, snapshotName)` → `multipass delete --purge <vm>.<snap>`
- `ExecInVM(vmName, commandArgs...)` → `multipass exec <vm> -- <args>`
- `ShellVM(vmName)` → attaches stdin/stdout directly. **Cannot work over HTTP.** Replace with websocket + PTY approach for the web UI.
- `ScanCloudInitFiles()` — finds local `.yml`/`.yaml` starting with `#cloud-config`
- `GetAllCloudInitTemplateOptions()` — aggregates local + GitHub repo templates
- `CloneRepoAndScanYAMLs(repoURL)` — shallow git clone for repo templates
- `ReadConfigGithubRepo()` — reads `.config` for GitHub template repo URL
- `CleanupTempDirs(dirs)` — cleanup after repo cloning

Types to reuse: `NetworkInfo`, `TemplateOption`

**`parsing.go` (137 lines) — Output parsers**

- `VMInfo` struct — Name, State, Snapshots, IPv4, Release, CPUs, Load, DiskUsage, MemoryUsage, Mounts
- `SnapshotInfo` struct — Instance, Name, Parent, Comment
- `parseVMInfo(info string) VMInfo` — parses `multipass info` text output (colon-separated key-value)
- `parseVMNames(listOutput string) []string` — parses `multipass list` text output
- `parseSnapshots(output string) []SnapshotInfo` — parses `multipass list --snapshots` text output

**Note:** These text parsers work but are fragile. Where possible, prefer `--format json` output. The `mount_operations.go` file shows how to do this properly.

**`mount_operations.go` (67 lines) — Mount management via JSON**

This is the **best pattern in the codebase** — it uses `multipass info <vm> --format json` and proper Go JSON unmarshalling. Use this approach as the model for other commands too.

- `MountInfo` struct — SourcePath, TargetPath, UIDMaps, GIDMaps
- `getVMMounts(vmName)` — JSON-based mount parsing
- JSON types: `multipassInfoResponse`, `multipassVMInfoDetail`, `multipassMountDetail`

**`constants.go` (77 lines) — Defaults and limits**

Reuse (skip the LLM constants):

- `DefaultUbuntuRelease = "24.04"`
- `DefaultCPUCores = 2`, `DefaultRAMMB = 1024`, `DefaultDiskGB = 8`
- `MinCPUCores = 1`, `MinRAMMB = 512`, `MinDiskGB = 1`
- `VMNamePrefix = "VM-"`, `VMNameRandomLength = 4`
- `UbuntuReleases = ["22.04", "20.04", "18.04", "24.04", "daily"]`

**`utils.go` — Helpers**

Reuse `randomString(length int) string` only (uses `crypto/rand`, generates names like `VM-a1b2`).

**`version.go` — Build metadata pattern**

Reuse the `Version`, `BuildTime`, `GitCommit` var pattern with `-ldflags`.

### 3.2 Files to use as REFERENCE ONLY

**`messages.go` (282 lines) — Operation catalogue**

Do not reuse the Bubble Tea `tea.Cmd`/`tea.Msg` types. But use this file as the **definitive list of every operation** the app supports. Each command factory maps to an API endpoint (see the API section below).

Key function to study: `doFetchVMList()` — calls `ListVMs()` to get names, then loops `GetVMInfo(name)` per VM. Also `runBulkVMOperation()` which runs ops on multiple VMs and aggregates errors with `errors.Join`.

**`ARCHITECTURE.md` — Design overview**

Read the "File Map" and "Message Flow" sections for understanding what features exist. Skip the "LLM Chat Integration" section.

**`view_create.go` — Create form fields (lines 52-78)**

Shows how the create form is assembled: cloud-init template scanning, network options building (NAT default, bridged fallback for Linux LXD, specific interface names with descriptions). Useful as reference for your web create form, but do not reuse the TUI code.

**`Taskfile.yml` — Build patterns**

Shows the cross-compilation commands and ldflags patterns. Reuse the `build-all` approach.

### 3.3 Files to IGNORE COMPLETELY

TUI code (Bubble Tea / Lipgloss):
`main.go`, `view_table.go`, `view_info.go`, `view_create.go`, `view_snapshots.go`, `view_mounts.go`, `view_modals.go`, `view_loading.go`, `view_chat.go`, `view_llm_settings.go`, `styles.go`, `themes.go`

LLM/AI features:
`agent.go`, `llm.go`, `config_llm.go`, `mcp_client.go`, `mcp_install.go`, `chat_messages.go`

Empty stubs:
`vm_operations.go` (2 lines), `snapshot_operations.go` (2 lines)

Tests (write fresh ones):
All `*_test.go` and `*_remediation_test.go` files

Deps:
`go.mod` / `go.sum` — start a fresh module

---

## 4. Architecture

### 4.1 Project Structure

```
passgo-web/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point: flags, config, HTTP server, embed
├── pkg/
│   └── multipass/
│       ├── client.go               # Client struct, logger, runCommand
│       ├── vms.go                  # List, Info, Start, Stop, Suspend, Delete, Recover, Launch
│       ├── snapshots.go            # Create, List, Restore, Delete snapshots
│       ├── mounts.go               # List, Add, Remove mounts (JSON parsing)
│       ├── networks.go             # ListNetworks
│       ├── cloudinit.go            # Template scanning, repo cloning
│       ├── types.go                # VMInfo, SnapshotInfo, MountInfo, NetworkInfo, etc.
│       └── constants.go            # Defaults, limits, releases
├── internal/
│   ├── api/
│   │   ├── handlers_vms.go        # VM CRUD handlers
│   │   ├── handlers_snapshots.go  # Snapshot handlers
│   │   ├── handlers_mounts.go     # Mount handlers
│   │   ├── handlers_system.go     # Networks, cloud-init templates, version, auth
│   │   ├── handlers_shell.go      # WebSocket shell handler
│   │   ├── middleware.go           # Auth, logging, CORS, JSON content-type
│   │   ├── routes.go              # Route registration
│   │   └── responses.go           # Standard JSON response helpers
│   ├── auth/
│   │   └── auth.go                # Username/password validation, session/token
│   └── config/
│       └── config.go              # Config file loading, CLI flags, defaults
├── frontend/
│   ├── index.html
│   ├── package.json
│   ├── vite.config.js
│   ├── tailwind.config.js
│   ├── postcss.config.js
│   ├── src/
│   │   ├── main.js                # Vue app bootstrap
│   │   ├── App.vue                # Root layout
│   │   ├── api/
│   │   │   └── client.js          # API client (one function per endpoint)
│   │   ├── stores/
│   │   │   └── vmStore.js         # Pinia store for VM state
│   │   ├── composables/
│   │   │   ├── usePolling.js      # Interval-based polling with pause/resume
│   │   │   └── useWebSocket.js    # WebSocket connection for shell
│   │   ├── components/
│   │   │   ├── layout/            # AppHeader, StatusBar, TreeSidebar
│   │   │   ├── host/              # HostPanel
│   │   │   ├── vm/                # VmDetailPanel, tabs (Summary, Console, etc.)
│   │   │   ├── modals/            # CreateVmModal, ConfirmModal, CreateSnapshotModal
│   │   │   └── shared/            # StatusDot, ActionButton, Toast
│   │   └── assets/
│   │       └── main.css           # Tailwind directives + dark theme CSS vars
│   └── dist/                      # Vite build output (embedded into Go binary)
├── go.mod
├── Makefile                        # (or Taskfile.yml)
└── README.md
```

### 4.2 Backend Design

**HTTP Server:** Use Go's `net/http` with Go 1.22+ routing patterns (supports `{name}` path params natively). If path matching becomes awkward, add `github.com/go-chi/chi/v5` — it's lightweight and idiomatic. Serve on a configurable port (default `:8080`).

**Multipass Client:** A `multipass.Client` struct holding a logger and optional config (e.g., custom multipass binary path). All CLI interactions are methods on this struct. No globals.

**Embedded Frontend:** Use `//go:embed frontend/dist/*` in the server main. Serve at `/` with a catch-all for SPA routing (serve `index.html` for any non-API, non-file path).

**Authentication:** Basic auth or a simple session token. On first run, if no config exists, prompt for or generate a username/password and write to `~/.passgo-web/config.json`. Every API request must include credentials (Basic auth header or a session cookie). The login page is the only unauthenticated route.

**Config File:** `~/.passgo-web/config.json` containing:
```json
{
  "listen": ":8080",
  "username": "admin",
  "password_hash": "<bcrypt hash>",
  "cloud_init_dir": "",
  "cloud_init_repo": ""
}
```

### 4.3 REST API

All endpoints return JSON. All mutating endpoints require authentication. Prefix: `/api/v1/`.

**VM Operations:**

| Method | Path | Body | Description |
|--------|------|------|-------------|
| GET | `/api/v1/vms` | — | List all VMs with full details |
| GET | `/api/v1/vms/{name}` | — | Get single VM details |
| POST | `/api/v1/vms` | `{name?, release?, cpus?, memoryMB?, diskGB?, cloudInit?, network?}` | Create VM. All fields optional (defaults applied). |
| POST | `/api/v1/vms/{name}/start` | — | Start VM |
| POST | `/api/v1/vms/{name}/stop` | — | Stop VM |
| POST | `/api/v1/vms/{name}/suspend` | — | Suspend VM |
| DELETE | `/api/v1/vms/{name}` | `{purge?: bool}` | Delete VM (optionally purge) |
| POST | `/api/v1/vms/{name}/recover` | — | Recover deleted VM |
| POST | `/api/v1/vms/start-all` | — | Start all stopped VMs |
| POST | `/api/v1/vms/stop-all` | — | Stop all running VMs |
| POST | `/api/v1/vms/purge` | — | Purge all deleted VMs |

**Snapshot Operations:**

| Method | Path | Body | Description |
|--------|------|------|-------------|
| GET | `/api/v1/vms/{name}/snapshots` | — | List snapshots for a VM |
| POST | `/api/v1/vms/{name}/snapshots` | `{name, comment?}` | Create snapshot |
| POST | `/api/v1/vms/{name}/snapshots/{snap}/restore` | — | Restore snapshot (destructive) |
| DELETE | `/api/v1/vms/{name}/snapshots/{snap}` | — | Delete snapshot |

**Mount Operations:**

| Method | Path | Body | Description |
|--------|------|------|-------------|
| GET | `/api/v1/vms/{name}/mounts` | — | List mounts for a VM |
| POST | `/api/v1/vms/{name}/mounts` | `{source, target}` | Add mount |
| DELETE | `/api/v1/vms/{name}/mounts` | `{target}` | Remove mount |

**System:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/networks` | List available network interfaces |
| GET | `/api/v1/cloud-init/templates` | List available cloud-init templates |
| GET | `/api/v1/version` | Server version info |
| POST | `/api/v1/auth/login` | Login, returns session token |
| POST | `/api/v1/auth/logout` | Invalidate session |

**Shell (WebSocket):**

| Path | Description |
|------|-------------|
| `/api/v1/vms/{name}/shell` | WebSocket endpoint. Spawns `multipass shell {name}` with a PTY, pipes bidirectionally. Frontend connects with xterm.js. |

**Exec (alternative to shell for simple commands):**

| Method | Path | Body | Description |
|--------|------|------|-------------|
| POST | `/api/v1/vms/{name}/exec` | `{command: ["ls", "-la"]}` | Run a command in VM, return stdout/stderr |

### 4.4 Frontend Design

**Technology: Vue 3 + Vite + Tailwind CSS**

This is a stateful, interactive UI (persistent tree sidebar, tabbed panels, modals, WebSocket terminal, polling refresh) — a component framework earns its keep here. Vue 3 with Composition API gives reactive state management, smooth transitions, and single-file components that map cleanly to the UI pieces. Vite provides fast dev-server hot reload and produces an optimised `dist/` folder for embedding.

**Stack:**
- **Vue 3** (Composition API with `<script setup>`) — component framework
- **Vite** — build tool and dev server
- **Tailwind CSS v3** — utility-first styling. Use the `@tailwindcss/forms` plugin for form inputs.
- **Vue Router** — not needed. This is a single-page app with no URL routing — the tree selection drives the detail panel, not the URL bar.
- **Pinia** — lightweight state store for shared VM data. One store (`useVmStore`) holds the VM list, selected node, and polling state. Components read reactively from the store.
- **xterm.js + @xterm/addon-fit + @xterm/addon-attach** — terminal emulator for the Console tab.
- **Lucide icons (via `lucide-vue-next`)** — clean, consistent icon set. Use for status indicators, action buttons, tree expand/collapse.

No additional UI component library. Build the components directly with Tailwind — it keeps the bundle small and avoids fighting a library's opinions.

**Frontend project setup:**

```
frontend/
├── index.html
├── package.json
├── vite.config.js
├── tailwind.config.js
├── postcss.config.js
├── src/
│   ├── main.js                     # createApp, mount, global polling setup
│   ├── App.vue                     # Root layout: header, sidebar, detail panel, status bar
│   ├── api/
│   │   └── client.js               # Thin wrapper around fetch() for all API calls
│   ├── stores/
│   │   └── vmStore.js              # Pinia store: VM list, selected node, polling
│   ├── composables/
│   │   ├── usePolling.js           # setInterval wrapper with pause/resume
│   │   └── useWebSocket.js         # WebSocket connection management for shell
│   ├── components/
│   │   ├── layout/
│   │   │   ├── AppHeader.vue       # Logo, hostname, user, logout
│   │   │   ├── StatusBar.vue       # Connection status, last refresh timestamp
│   │   │   └── TreeSidebar.vue     # Host + VM tree with status dots
│   │   ├── host/
│   │   │   └── HostPanel.vue       # Summary cards, bulk actions, quick create
│   │   ├── vm/
│   │   │   ├── VmDetailPanel.vue   # Tab container for selected VM
│   │   │   ├── VmSummaryTab.vue    # Info + action buttons
│   │   │   ├── VmConsoleTab.vue    # xterm.js terminal
│   │   │   ├── VmSnapshotsTab.vue  # Snapshot table + CRUD
│   │   │   ├── VmMountsTab.vue     # Mount table + add/remove
│   │   │   └── VmConfigTab.vue     # Raw info display
│   │   ├── modals/
│   │   │   ├── CreateVmModal.vue   # Advanced create form
│   │   │   ├── ConfirmModal.vue    # Generic yes/no confirmation
│   │   │   └── CreateSnapshotModal.vue
│   │   └── shared/
│   │       ├── StatusDot.vue       # Coloured dot component (green/yellow/grey/red)
│   │       ├── ActionButton.vue    # Icon button with loading state
│   │       └── Toast.vue           # Notification toast (success/error)
│   └── assets/
│       └── main.css                # Tailwind directives + custom dark theme vars
└── dist/                           # Built output (embedded into Go binary)
```

**API client (`src/api/client.js`):**

A simple module exporting one function per API endpoint. Every function returns a Promise. Handles auth token attachment, JSON parsing, and error normalisation. Example shape:

```javascript
const API_BASE = '/api/v1'

async function request(method, path, body) {
  const res = await fetch(API_BASE + path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
  return res.json()
}

export const listVMs = () => request('GET', '/vms')
export const getVM = (name) => request('GET', `/vms/${name}`)
export const createVM = (opts) => request('POST', '/vms', opts)
export const startVM = (name) => request('POST', `/vms/${name}/start`)
export const stopVM = (name) => request('POST', `/vms/${name}/stop`)
// ... etc for every endpoint
```

**Pinia store (`src/stores/vmStore.js`):**

Single store managing the VM list and selection state. The polling composable calls `listVMs()` on an interval and writes results into the store. All components read from here reactively.

```javascript
export const useVmStore = defineStore('vms', {
  state: () => ({
    vms: [],
    selectedNode: null,     // null = host, string = VM name
    lastRefresh: null,
    loading: false,
    error: null,
  }),
  getters: {
    selectedVm: (state) => state.vms.find(vm => vm.name === state.selectedNode),
    runningCount: (state) => state.vms.filter(vm => vm.state === 'Running').length,
    stoppedCount: (state) => state.vms.filter(vm => vm.state === 'Stopped').length,
    // etc.
  },
  actions: {
    async fetchVMs() { /* call listVMs(), update state */ },
    selectNode(name) { this.selectedNode = name },
  },
})
```

**Dark theme and visual style:**

Use a dark colour scheme to match the Proxmox/infrastructure-tool aesthetic. Define a small set of CSS custom properties in `main.css` and reference them via Tailwind's `extend` config:

```css
/* main.css */
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --bg-primary: #1a1a2e;       /* deep navy main background */
  --bg-secondary: #16213e;     /* sidebar and cards */
  --bg-surface: #1e293b;       /* panels, modals */
  --bg-hover: #2a3a5c;         /* hover states */
  --border: #334155;            /* subtle borders */
  --text-primary: #e2e8f0;     /* main text */
  --text-secondary: #94a3b8;   /* muted text */
  --accent: #3b82f6;           /* blue accent for active states, buttons */
  --success: #22c55e;          /* running status */
  --warning: #eab308;          /* suspended status */
  --danger: #ef4444;           /* delete, errors */
  --muted: #64748b;            /* stopped status */
}
```

**Transitions and polish:**
- `<Transition>` on detail panel swap — quick fade (150ms) when switching between host and VM panels
- `<TransitionGroup>` on the tree list — smooth insert/remove when VMs are created or deleted
- Loading spinners on action buttons — button shows a spinner icon and disables while the API call is in flight
- Toast notifications — slide in from top-right on success/error, auto-dismiss after 4 seconds
- Status dots pulse gently for "Starting" / "Suspending" transitional states
- Tab underline slides smoothly between tabs (CSS transition on the indicator element)

Keep it tasteful — these are small touches, not animations for the sake of it.

**Layout — Proxmox/vSphere pattern:**

```
┌─────────────────────────────────────────────────────┐
│  Header: PassGo Web  |  hostname  |  user  | logout │
├──────────┬──────────────────────────────────────────┤
│          │                                           │
│  Tree    │  Detail Panel                             │
│          │                                           │
│  ▼ Host  │  (changes based on tree selection)        │
│    ├ vm1 │                                           │
│    ├ vm2 │  When host selected:                      │
│    └ vm3 │    - Summary cards (total, running, etc.) │
│          │    - Bulk actions (start all, stop all)   │
│          │    - Create VM button                     │
│          │                                           │
│          │  When VM selected:                        │
│          │    - Tabbed interface:                     │
│          │      Summary | Console | Snapshots |      │
│          │      Mounts | Config                      │
│          │                                           │
├──────────┴──────────────────────────────────────────┤
│  Status bar: connected | 5 VMs | last refresh: 3s   │
└─────────────────────────────────────────────────────┘
```

**Tree sidebar (`TreeSidebar.vue`):**
- Root node = the host machine (hostname from `GET /api/v1/version` or "localhost")
- Child nodes = VMs, each showing a `<StatusDot>` (green=Running, yellow=Suspended, grey=Stopped, red=Deleted)
- Selected node has a highlighted background (accent colour with low opacity)
- Tree updates reactively from the Pinia store — no manual refresh logic in the component
- Right-click context menu on VM nodes (optional, v1 stretch goal) for quick actions

**Host detail panel (`HostPanel.vue`, shown when root node selected):**
- Summary cards in a grid: total VMs, running, stopped, suspended, deleted — each with an icon and count
- Action bar: Create VM (opens modal), Start All, Stop All, Purge Deleted
- Each bulk action button shows a confirmation modal before executing

**VM detail panel (`VmDetailPanel.vue`, shown when a VM is selected) — tabbed:**

- **Summary tab (`VmSummaryTab.vue`):** Two-column layout. Left: property list (name, state, IP, release, image, CPUs, memory usage, disk usage, load). Right: action buttons stacked vertically (Start, Stop, Suspend, Delete, Recover — contextually enabled based on VM state). State shown as a coloured badge.
- **Console tab (`VmConsoleTab.vue`):** Full-height embedded terminal using xterm.js. Connects to `ws://host/api/v1/vms/{name}/shell` on tab activation, disconnects on tab deactivation. Shows a reconnect button and connection status indicator. Use the `FitAddon` to fill available space.
- **Snapshots tab (`VmSnapshotsTab.vue`):** Table with columns: Name, Parent, Comment. Action buttons per row: Restore, Delete. "Create Snapshot" button at the top opens `CreateSnapshotModal`. If VM is running, show a warning banner: "Stop the VM to manage snapshots" and disable the buttons.
- **Mounts tab (`VmMountsTab.vue`):** Table with columns: Source Path, Target Path, UID Maps, GID Maps. "Add Mount" button (opens a small inline form with source and target fields). Remove button per row with confirmation.
- **Config tab (`VmConfigTab.vue`):** Monospace pre-formatted display of the raw `multipass info` output. Read-only for v1.

**Create VM modal (`CreateVmModal.vue`):**
- Overlay modal with backdrop blur
- Form fields (all styled with Tailwind + `@tailwindcss/forms`):
  - Name (text, placeholder shows auto-generated default like `VM-a1b2`)
  - Ubuntu release (select dropdown: 24.04, 22.04, 20.04, 18.04, daily — default 24.04)
  - CPUs (number input, default 2, min 1)
  - RAM MB (number input, default 1024, min 512)
  - Disk GB (number input, default 8, min 1)
  - Cloud-init template (select dropdown, populated from `/api/v1/cloud-init/templates`, default "None")
  - Network (select dropdown, populated from `/api/v1/networks`, default "Default (NAT)")
- Create button with loading spinner during API call
- Cancel button
- On success: close modal, show success toast, new VM appears in tree automatically via next poll

**State updates and polling:**
- The `usePolling` composable calls `vmStore.fetchVMs()` every 3 seconds
- Polling pauses when the browser tab is hidden (`document.visibilitychange`) and resumes when visible
- After any mutating action (start, stop, create, delete), trigger an immediate fetch rather than waiting for the next poll interval
- The selected VM's detail panel re-renders reactively when the store updates — no separate fetch needed per tab (except Console which is its own WebSocket connection)

**Dev workflow:**
```bash
cd frontend
npm install
npm run dev        # Vite dev server on :5173, proxies /api to Go backend on :8080
npm run build      # Produces dist/ for embedding
npm run preview    # Preview production build locally
```

**Vite config (`vite.config.js`):**
```javascript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        ws: true,      // proxy WebSocket connections too
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
```

---

## 5. Build & Distribution

### 5.1 Build Process

```makefile
# Install frontend dependencies
frontend-install:
    cd frontend && npm install

# Build frontend (produces frontend/dist/)
frontend-build: frontend-install
    cd frontend && npm run build

# Dev mode: run Go backend + Vite dev server (with hot reload + API proxy)
dev:
    # Terminal 1: go run ./cmd/server --port 8080
    # Terminal 2: cd frontend && npm run dev
    @echo "Run 'go run ./cmd/server' in one terminal and 'cd frontend && npm run dev' in another"

# Build Go binary (embeds frontend/dist)
build: frontend-build
    go build -ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(DATE) -X main.GitCommit=$(COMMIT)" \
        -o passgo-web ./cmd/server

# Cross-compile all platforms
build-all: frontend-build
    GOOS=darwin  GOARCH=arm64 go build -ldflags="..." -o dist/passgo-web-darwin-arm64  ./cmd/server
    GOOS=darwin  GOARCH=amd64 go build -ldflags="..." -o dist/passgo-web-darwin-amd64  ./cmd/server
    GOOS=linux   GOARCH=arm64 go build -ldflags="..." -o dist/passgo-web-linux-arm64   ./cmd/server
    GOOS=linux   GOARCH=amd64 go build -ldflags="..." -o dist/passgo-web-linux-amd64   ./cmd/server
    GOOS=windows GOARCH=amd64 go build -ldflags="..." -o dist/passgo-web-windows-amd64.exe ./cmd/server
    upx dist/passgo-web-linux-*
    upx dist/passgo-web-windows-*
```

### 5.2 Running

```bash
# First run — prompts for or auto-generates credentials
./passgo-web

# With options
./passgo-web --port 9090 --config /path/to/config.json

# Output:
# PassGo Web v0.1.0
# Config: ~/.passgo-web/config.json
# Listening on http://127.0.0.1:8080
# Default credentials — admin / <generated-password>
```

### 5.3 CLI Flags

```
--port        Listen port (default: 8080)
--config      Config file path (default: ~/.passgo-web/config.json)
--no-auth     Disable authentication (for local-only use)
--version     Print version and exit
```

---

## 6. Adaptation Notes

These are specific technical gotchas discovered from reading the PassGo source code:

1. **`ShellVM()` cannot work over HTTP.** It attaches directly to stdin/stdout via `os.Stdin`/`os.Stdout`. For the web UI, use `github.com/creack/pty` to spawn `multipass shell {name}` with a pseudo-terminal, then pipe the PTY file descriptor bidirectionally over a `github.com/gorilla/websocket` connection. On the frontend, use `xterm.js` with its WebSocket addon.

2. **Prefer `--format json` over text parsing.** The text parsers in `parsing.go` split on whitespace and colons — they break if field values contain colons or unusual whitespace. `mount_operations.go` demonstrates the correct JSON approach using `multipass info <vm> --format json`. Use this pattern for `list`, `info`, and `networks` too.

3. **The VM list fetch is O(n) sequential CLI calls.** `doFetchVMList()` in `messages.go` calls `ListVMs()` then loops `GetVMInfo(name)` per VM. For the web API, either use `multipass info --all --format json` (single call) or at minimum parallelise with goroutines and a `sync.WaitGroup`.

4. **Cloud-init template scanning** in PassGo looks in the directory where the binary lives (`os.Executable()` path) and the current working directory. For a web server, make this a configurable path in the config file instead.

5. **The auto-refresh interval** in the TUI is 1 second. For the web UI, 3-5 seconds is more appropriate to avoid hammering the multipass CLI.

6. **Network listing can fail.** `ListNetworks()` returns an error on some platforms (e.g., Linux with LXD backend). The create form should gracefully fall back to showing "Default (NAT)" and "Bridged (default)" options when this happens, as `view_create.go` lines 65-78 demonstrate.

7. **Snapshots require stopped VMs.** The UI must communicate this clearly — disable snapshot create/restore buttons when the VM is running, with a tooltip explaining why.

8. **Windows PTY considerations.** `github.com/creack/pty` does not work on Windows. For cross-platform shell support, consider `github.com/iamacarpet/go-winpty` on Windows or fall back to `ExecInVM()` (non-interactive command execution) when PTY is unavailable.

---

## 7. Testing Strategy

The test suite must be comprehensive enough that an AI agent can run tests, see failures, and iterate on fixes autonomously. This means: fast tests that run without multipass, clear error messages, and good coverage of the API surface.

### 7.1 Unit Tests (`pkg/multipass/`)

Test the parsing and type logic without requiring multipass to be installed.

```
pkg/multipass/
├── vms_test.go           # Test parseVMInfo, parseVMNames with sample outputs
├── snapshots_test.go     # Test parseSnapshots with sample outputs
├── mounts_test.go        # Test JSON mount parsing with sample JSON
├── cloudinit_test.go     # Test template scanning with temp directories
├── constants_test.go     # Validate defaults and limits
└── client_test.go        # Test Client construction, command building (mock exec)
```

**Key approach:** Create test fixtures with real multipass output captured from actual runs. Store as `testdata/` files or string constants. For example:

```go
// testdata/multipass_list.txt
Name                    State             IPv4             Image
vm1                     Running           10.72.73.1       Ubuntu 24.04 LTS
vm2                     Stopped           --               Ubuntu 22.04 LTS
vm3                     Deleted           --               Ubuntu 24.04 LTS

// testdata/multipass_info_vm1.txt
Name:           vm1
State:          Running
Snapshots:      2
IPv4:           10.72.73.1
Release:        Ubuntu 24.04 LTS
CPU(s):         2
Load:           0.08 0.02 0.01
Disk usage:     1.5GiB out of 7.7GiB
Memory usage:   218.4MiB out of 962.3MiB
Mounts:         /home/user/shared => /mnt/shared

// testdata/multipass_info_json.json
{"errors":[],"info":{"vm1":{"mounts":{"/mnt/shared":{"source_path":"/home/user/shared","gid_mappings":["1000:default"],"uid_mappings":["1000:default"]}}}}}
```

**Mock the CLI executor** by making `runCommand` a field on the `Client` struct (a function type). Tests inject a fake that returns canned output:

```go
type CommandRunner func(args ...string) (string, error)

type Client struct {
    logger *slog.Logger
    run    CommandRunner
}

// In tests:
client := &Client{
    run: func(args ...string) (string, error) {
        if args[0] == "list" {
            return testdata.MultipassList, nil
        }
        return "", fmt.Errorf("unexpected command: %v", args)
    },
}
```

### 7.2 API Integration Tests (`internal/api/`)

Test every HTTP endpoint with a mock multipass client. No real VMs needed.

```
internal/api/
├── handlers_vms_test.go
├── handlers_snapshots_test.go
├── handlers_mounts_test.go
├── handlers_system_test.go
├── handlers_shell_test.go     # WebSocket connection test
└── middleware_test.go          # Auth, CORS
```

**Use `httptest.NewServer`** to spin up the full router with a mock multipass client:

```go
func setupTestServer(t *testing.T) (*httptest.Server, *mock.MultipassClient) {
    mockClient := mock.NewMultipassClient()
    handler := api.NewRouter(mockClient, testLogger, testAuth)
    return httptest.NewServer(handler), mockClient
}

func TestListVMs(t *testing.T) {
    srv, mock := setupTestServer(t)
    defer srv.Close()
    
    mock.SetVMs([]multipass.VMInfo{
        {Name: "vm1", State: "Running", IPv4: "10.0.0.1"},
        {Name: "vm2", State: "Stopped"},
    })
    
    resp, err := http.Get(srv.URL + "/api/v1/vms")
    // assert status 200, body contains 2 VMs, correct fields
}
```

**Test every endpoint** for: success case, not-found VM, multipass error, invalid input, auth required, auth rejected.

### 7.3 End-to-End Tests (optional, requires multipass)

These require a machine with multipass installed and are slow. Guard behind a build tag:

```go
//go:build e2e

func TestE2E_CreateAndDeleteVM(t *testing.T) {
    // Start real server
    // POST /api/v1/vms with name "test-e2e-xxx"
    // Poll GET /api/v1/vms/{name} until Running
    // POST /api/v1/vms/{name}/stop
    // DELETE /api/v1/vms/{name}?purge=true
    // Verify gone
}
```

Run with: `go test -tags e2e -timeout 300s ./...`

### 7.4 Frontend Tests

Use **Vitest** (installed as a development dependency) for unit tests of Vue components, composables, stores, and the API client.

```
frontend/
├── src/
│   ├── api/__tests__/
│   │   └── client.test.js          # Mock fetch, verify URLs and request bodies
│   ├── stores/__tests__/
│   │   └── vmStore.test.js         # Store actions, getters, state mutations
│   └── components/__tests__/
│       ├── TreeSidebar.test.js     # Given VM list, renders correct nodes + status dots
│       ├── CreateVmModal.test.js   # Form validation (min values, required fields)
│       └── VmSummaryTab.test.js    # Correct buttons enabled/disabled per VM state
```

**What to test:**
- **API client** — mock `global.fetch`, verify each exported function calls the correct URL with the correct method and body
- **Pinia store** — test `fetchVMs` action updates state, test getters compute counts correctly, test `selectNode` sets selection
- **TreeSidebar** — mount with a mock store containing 3 VMs in different states, assert correct number of nodes rendered, correct status colours
- **CreateVmModal** — test that form enforces min CPU/RAM/disk values, that submitting calls the API client, that cancel emits close
- **VmSummaryTab** — test that Start button is disabled when VM is Running, Delete shows confirmation, etc.

**Test command:**
```bash
cd frontend && npm test                # Single run
cd frontend && npx vitest              # Watch mode
```

Add to the top-level Makefile:
```makefile
test-frontend:
    cd frontend && npm test
```

### 7.5 Test Commands

```makefile
# Run all fast Go tests (no multipass needed)
test:
    go test ./... -v -count=1

# Run frontend tests
test-frontend:
    cd frontend && npm test

# Run with race detector
test-race:
    go test ./... -v -race -count=1

# Run with coverage
test-cover:
    go test ./... -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Run end-to-end tests (requires multipass)
test-e2e:
    go test -tags e2e -timeout 300s ./...

# Run everything
test-all: test test-frontend test-race test-e2e

# Lint and security
lint:
    golangci-lint run ./...
    cd frontend && npx vue-tsc --noEmit

security:
    gosec -quiet ./...
    govulncheck ./...
```

### 7.6 CI-Friendly Test Design

Tests must be **deterministic** and **parallelisable**:
- No global state — each test creates its own mock client
- No port conflicts — use `httptest.NewServer` (random port)
- No file system side effects — use `t.TempDir()` for any file operations
- Tests should complete in under 30 seconds total (excluding e2e)
- Every test should print a clear failure message explaining what went wrong and what was expected

---

## 8. Implementation Order

Build in this sequence so that each phase is testable before moving to the next:

### Phase 1: Multipass Package + Tests
1. Create `pkg/multipass/` with the Client struct and all CLI wrapper methods adapted from PassGo
2. Write unit tests with mocked CLI output for every parsing function
3. Verify: `go test ./pkg/multipass/... -v` passes

### Phase 2: REST API + Tests
1. Create `internal/api/` with all handlers, routes, middleware
2. Write API integration tests with mock multipass client
3. Verify: `go test ./internal/... -v` passes
4. Verify: can start server and `curl` every endpoint manually

### Phase 3: Authentication
1. Implement basic auth middleware and config file
2. Add auth tests
3. Verify: unauthenticated requests get 401

### Phase 4: Frontend — Scaffold + Tree + Dashboard
1. Scaffold the Vue 3 project: `npm create vite@latest frontend -- --template vue`, add Tailwind CSS, Pinia, lucide-vue-next
2. Set up `vite.config.js` with API proxy to Go backend on `:8080`
3. Create `App.vue` with the three-zone layout (header, sidebar, detail panel) using Tailwind grid/flex
4. Set up the dark theme CSS variables in `main.css`
5. Build `TreeSidebar.vue` — hardcode some fake VMs first, then connect to the Pinia store
6. Build `vmStore.js` with `fetchVMs` action calling the API client
7. Build `api/client.js` with `listVMs()` and `getVM()` to start
8. Build `usePolling.js` composable, wire it into `App.vue` to poll every 3 seconds
9. Build `HostPanel.vue` with summary cards and bulk action buttons
10. Verify: `npm run dev` shows the tree updating live as VMs are started/stopped via CLI

### Phase 5: Frontend — VM Detail Tabs
1. Build `VmDetailPanel.vue` with tab navigation (Summary, Console, Snapshots, Mounts, Config)
2. Build `VmSummaryTab.vue` — property list + contextual action buttons
3. Build `CreateVmModal.vue` — form with all fields, validation, API submission
4. Build `VmSnapshotsTab.vue` — table + create/restore/delete with `CreateSnapshotModal.vue`
5. Build `VmMountsTab.vue` — table + add/remove
6. Build `VmConfigTab.vue` — raw info display
7. Add `Toast.vue` and `ConfirmModal.vue` for feedback and destructive action confirmation
8. Add `<Transition>` on panel/tab switches and `<TransitionGroup>` on the tree list
9. Write Vitest tests for the API client, store, and key components
10. Verify: `npm test` passes, full workflow works in browser

### Phase 6: Frontend — Console
1. Install xterm.js, @xterm/addon-fit, @xterm/addon-attach
2. Build `useWebSocket.js` composable — connect/disconnect/reconnect logic
3. Build `VmConsoleTab.vue` — xterm.js instance, connects on tab activation, FitAddon for responsive sizing
4. Add connection status indicator and reconnect button
5. Verify: can type commands in the browser terminal and see output

### Phase 7: Build & Distribution
1. Embed frontend into binary
2. Cross-compilation Makefile
3. First-run experience (credential generation)
4. README with installation instructions

---

## 9. Non-Goals (for v1)

- No HTTPS built-in (use a reverse proxy if needed)
- No multi-user / RBAC
- No multipass daemon management (assumes multipass is already installed and running)
- No mobile-optimised layout (desktop browsers only)
- No dark/light theme toggle (dark theme only — matches the Proxmox/infrastructure-tool aesthetic)
- No persistent database (all state comes from multipass CLI on each request)
- No cluster management (single host only)
