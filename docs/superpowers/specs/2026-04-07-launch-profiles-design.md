# Launch Profiles + Auto-Run Playbooks

## Context

PassGo Web's most common workflow is: create VM → configure it with Ansible. Today these are separate manual steps — fill out the create modal, wait for launch, switch to the Ansible tab, select a playbook, pick the target, run it. For VMs you create repeatedly (dev environments, k8s nodes, test boxes), this is repetitive friction.

Launch profiles let you save a complete VM recipe (image, specs, cloud-init, network, group, and an optional Ansible playbook) and launch it in one click. The playbook auto-runs after the VM comes up.

## Data Model

### Profile struct (Go)

```go
type Profile struct {
    ID        string `json:"id"`         // URL-safe identifier (e.g., "k8s-node")
    Name      string `json:"name"`       // Display name (e.g., "Kubernetes Node")
    Release   string `json:"release"`    // Image/blueprint (e.g., "24.04")
    CPUs      int    `json:"cpus"`       // 0 = use vm_defaults
    MemoryMB  int    `json:"memory_mb"`  // 0 = use vm_defaults
    DiskGB    int    `json:"disk_gb"`    // 0 = use vm_defaults
    CloudInit string `json:"cloud_init"` // Template name, "" = none
    Network   string `json:"network"`    // "" = default (NAT)
    Playbook  string `json:"playbook"`   // Playbook filename, "" = none
    Group     string `json:"group"`      // Group name, "" = ungrouped
}
```

### Storage

Stored as `profiles` array in `~/.passgo-web/config.json`, alongside existing `vm_defaults`, `groups`, and `vm_groups`. Protected by the existing config mutex.

```json
{
  "vm_defaults": { ... },
  "groups": ["kubernetes", "dev"],
  "vm_groups": { ... },
  "profiles": [
    {
      "id": "k8s-node",
      "name": "Kubernetes Node",
      "release": "24.04",
      "cpus": 2,
      "memory_mb": 4096,
      "disk_gb": 20,
      "cloud_init": "builtin:docker",
      "network": "",
      "playbook": "setup-k8s.yml",
      "group": "kubernetes"
    }
  ]
}
```

### Validation

- `id`: required, `[a-zA-Z0-9_-]+`, unique across profiles
- `name`: required, non-empty display string
- `release`: optional (if empty, user must pick at launch time)
- `cpus`, `memory_mb`, `disk_gb`: 0 means "use vm_defaults"; if set, same minimums as VMDefaults (cpus≥1, memory≥512, disk≥1)
- `cloud_init`: if set, must be a valid template name (builtin: prefix or existing user template)
- `playbook`: if set, must exist in playbooks directory
- `group`: if set, must be an existing group name

## API Endpoints

### Profile CRUD

All under `/api/v1/profiles`, protected by auth middleware.

| Method | Path | Request Body | Response | Status |
|--------|------|-------------|----------|--------|
| GET | `/profiles` | — | `[Profile]` | 200 |
| POST | `/profiles` | `Profile` | `Profile` | 201 |
| PUT | `/profiles/{id}` | `Profile` (id in URL) | `Profile` | 200 |
| DELETE | `/profiles/{id}` | — | `{message}` | 200 |

**Error cases:**
- POST with duplicate `id` → 409 Conflict
- PUT/DELETE with unknown `id` → 404 Not Found
- Validation failure → 400 Bad Request with field-level error

### Modified VM Launch

`POST /api/v1/vms` gains an optional `profile` field:

```json
{
  "profile": "k8s-node",
  "name": "my-k8s-01",
  "cpus": 4
}
```

**Resolution order:**
1. Start with `vm_defaults` as base
2. Overlay profile values (non-zero/non-empty fields)
3. Overlay request values (non-zero/non-empty fields)

This lets the user pick a profile but override specific fields (e.g., more CPUs).

### Post-Launch Actions

After a successful `multipass launch` in the existing goroutine:

1. **Group assignment:** If resolved profile has `group` set, call `cfg.AssignVMGroup(vmName, group)` + `cfg.Save()`
2. **Playbook auto-run:** If resolved profile has `playbook` set, call `s.ansibleRunner.enqueue(playbook, []string{vmName})`

Both steps are fire-and-forget from the launch goroutine's perspective. Failures are surfaced through existing mechanisms (group via config save errors; playbook via ansible run status).

## Ansible Run Queue

### Current State

`ansibleRunner` supports exactly one run. Starting a second returns 409 Conflict.

### New Behavior

Add a FIFO queue to `ansibleRunner`:

```go
type queueEntry struct {
    Playbook string
    VMs      []string
}

type ansibleRunner struct {
    mu      sync.Mutex
    current *ansibleRun
    queue   []queueEntry  // new
    // ... existing fields
}
```

**Methods:**

- `enqueue(playbook, vms)` — appends to queue. If no current run, dequeues and starts immediately.
- `dequeueNext()` — called internally when a run finishes. Pops next entry, builds the ansible command, starts it.
- `cancel()` — kills current run. Does NOT clear the queue (next item starts automatically).
- `clearQueue()` — empties queue without affecting current run.

**Manual vs auto-run behavior:**
- Manual runs from `POST /ansible/run` still return 409 if a run is active. Users can see the queue and wait. This keeps the explicit UX clear.
- Auto-runs from launch profiles always enqueue silently.

### New Endpoint

| Method | Path | Response | Status |
|--------|------|----------|--------|
| GET | `/ansible/run/queue` | `[{playbook, vms}]` | 200 |
| DELETE | `/ansible/run/queue` | `{message}` | 200 |

## Frontend Changes

### CreateVmModal

**New elements:**
- Profile dropdown at the top of the form, above the name field
- Options: "(No profile)" + all profiles from `api.getProfiles()`
- Selecting a profile pre-fills form fields (release, cpus, memory, disk, cloud-init, network)
- Fields are still editable after profile selection
- "Save as Profile" button in the modal footer, next to "Create"
  - Opens a small inline form: ID + display name
  - Captures current form state as a new profile
  - Calls `POST /profiles`

**Behavior:**
- On mount: fetch profiles alongside images, networks, cloud-init templates (parallel)
- Profile selection does NOT lock the form — it's a convenience pre-fill
- If a profile has a playbook, show a small note below the dropdown: "Will auto-run: setup-k8s.yml"

### Profile Management

**Location:** New "Profiles" node in the tree sidebar, under the Settings section.

**Panel:** `ProfilesPanel.vue` — simple list + edit view:
- Left column: list of profiles with name and summary (e.g., "24.04 · 2 CPU · 4 GB · setup-k8s.yml")
- Right column: edit form (same fields as the profile struct)
- Delete button with confirmation
- No inline creation — use "Save as Profile" from CreateVmModal or a "New Profile" button in the panel

### Ansible Tab Queue Indicator

When the queue has entries:
- Small badge on the Ansible tab showing queue count
- In the run status area: "Queued: setup-k8s.yml → vm-01" with a cancel button per entry
- Queue clears as items execute

## Files to Modify

### Backend
- `internal/config/config.go` — Add `Profile` struct, `Profiles []Profile` to Config, CRUD methods
- `internal/api/handlers_profiles.go` — New file: profile CRUD handlers
- `internal/api/handlers_vms.go` — Modify `handleCreateVM` to accept `profile` field, resolve merged config, trigger post-launch actions
- `internal/api/ansible_runner.go` — Add queue field, `enqueue()`, `dequeueNext()`, queue endpoints
- `internal/api/handlers_ansible.go` — Add queue status/clear handlers
- `internal/api/routes.go` — Register new profile and queue endpoints
- `internal/api/server.go` — No changes expected (profiles accessed via config)

### Frontend
- `frontend/src/api/client.js` — Add profile API calls, queue endpoint
- `frontend/src/components/modals/CreateVmModal.vue` — Profile dropdown, "Save as Profile" button
- `frontend/src/components/profiles/ProfilesPanel.vue` — New file: profile management panel
- `frontend/src/components/TreeSidebar.vue` — Add "Profiles" node
- `frontend/src/components/vm/VmAnsibleTab.vue` — Queue indicator
- `frontend/src/stores/vmStore.js` — Add profiles state + fetch action (or a separate small store)

## Verification

1. **Profile CRUD:** Create, list, edit, delete profiles via the Profiles panel. Verify config.json updates.
2. **Launch from profile:** Select profile in CreateVmModal → verify form pre-fills → launch → verify VM specs match profile.
3. **Override from profile:** Select profile, change CPUs → launch → verify override applied.
4. **Auto group assignment:** Profile with group set → launch → verify VM appears in correct group in sidebar.
5. **Auto playbook run:** Profile with playbook → launch → verify playbook queues and executes after VM is up.
6. **Queue behavior:** Launch two VMs with playbooks in quick succession → verify both playbooks run sequentially.
7. **Manual + queue interaction:** Queue has pending auto-runs → manually run a playbook from Ansible tab → verify 409 returned (manual run doesn't jump queue).
8. **Queue management:** Verify queue is visible in Ansible tab, entries can be cancelled.
9. **Edge cases:** Launch with profile whose playbook was deleted → verify graceful error. Launch with profile whose group was deleted → verify VM created ungrouped with warning.
