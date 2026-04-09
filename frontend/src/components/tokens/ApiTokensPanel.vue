<script setup>
import { ref, onMounted } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { Plus, Trash2, Copy, Check, Download } from 'lucide-vue-next'
import ConfirmModal from '../modals/ConfirmModal.vue'

const toasts = useToastStore()

const activeTab = ref('tokens')
const tokens = ref([])
const loading = ref(true)

// Create form
const creating = ref(false)
const newName = ref('')
const saving = ref(false)

// Newly created token (shown once)
const newToken = ref(null)
const copied = ref(false)

// Delete confirmation
const confirmDelete = ref(null)

onMounted(async () => {
  try {
    const list = await api.listTokens()
    tokens.value = Array.isArray(list) ? list : []
  } catch { /* ignore */ }
  loading.value = false
})

function formatDate(ts) {
  try {
    return new Date(ts).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
  } catch { return ts }
}

async function createToken() {
  if (!newName.value.trim()) return
  saving.value = true
  try {
    const result = await api.createToken(newName.value.trim())
    newToken.value = result
    creating.value = false
    newName.value = ''
    tokens.value = await api.listTokens()
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}

async function copyToken() {
  if (!newToken.value) return
  try {
    await navigator.clipboard.writeText(newToken.value.token)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    toasts.error('Failed to copy to clipboard')
  }
}

function dismissToken() {
  newToken.value = null
  copied.value = false
}

async function doDelete() {
  if (!confirmDelete.value) return
  const { id, name } = confirmDelete.value
  confirmDelete.value = null
  try {
    await api.deleteToken(id)
    toasts.success(`Token "${name}" deleted`)
    tokens.value = await api.listTokens()
  } catch (e) {
    toasts.error(e.message)
  }
}
</script>

<template>
  <div class="h-full flex flex-col">
    <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
      <h2 class="text-lg font-semibold">API Tokens</h2>
      <div class="flex gap-1 bg-[var(--bg-primary)] rounded p-0.5 border border-[var(--border)]">
        <button
          @click="activeTab = 'tokens'"
          class="px-3 py-1 text-xs rounded transition-colors"
          :class="activeTab === 'tokens' ? 'bg-[var(--accent)] text-white' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'"
        >Tokens</button>
        <button
          @click="activeTab = 'guide'"
          class="px-3 py-1 text-xs rounded transition-colors"
          :class="activeTab === 'guide' ? 'bg-[var(--accent)] text-white' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'"
        >API Guide</button>
      </div>
    </div>

    <!-- Tokens Tab -->
    <div v-if="activeTab === 'tokens'" class="flex-1 overflow-y-auto">
      <div v-if="loading" class="flex-1 flex items-center justify-center text-[var(--text-secondary)] text-sm p-6">
        Loading...
      </div>

      <div v-else class="p-4 space-y-4">
        <!-- New token banner -->
        <div v-if="newToken" class="bg-[var(--accent)]/10 border border-[var(--accent)]/30 rounded-lg p-4">
          <div class="flex items-center justify-between mb-2">
            <span class="text-sm font-medium text-[var(--text-primary)]">Token created: {{ newToken.name }}</span>
            <button @click="dismissToken" class="text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)]">Dismiss</button>
          </div>
          <div class="flex items-center gap-2 mb-2">
            <code class="flex-1 bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm font-mono text-[var(--text-primary)] select-all break-all">{{ newToken.token }}</code>
            <button @click="copyToken"
              class="flex items-center gap-1 px-3 py-2 text-xs rounded bg-[var(--bg-primary)] border border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors flex-shrink-0"
              :class="copied ? 'text-[var(--success)]' : 'text-[var(--text-secondary)]'"
            >
              <Check v-if="copied" class="w-3.5 h-3.5" />
              <Copy v-else class="w-3.5 h-3.5" />
              {{ copied ? 'Copied' : 'Copy' }}
            </button>
          </div>
          <p class="text-xs text-[var(--warning,orange)]">Copy this token now. It won't be shown again.</p>
        </div>

        <!-- Create form -->
        <div v-if="creating" class="flex items-end gap-2">
          <div class="flex-1">
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Token Name</label>
            <input
              v-model="newName"
              type="text"
              placeholder="e.g. Home Assistant, CI/CD, monitoring script"
              maxlength="64"
              @keyup.enter="createToken"
              @keyup.escape="creating = false; newName = ''"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
          </div>
          <button @click="createToken" :disabled="!newName.trim() || saving"
            class="px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40">
            Create
          </button>
          <button @click="creating = false; newName = ''"
            class="px-3 py-1.5 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors">
            Cancel
          </button>
        </div>

        <button v-else @click="creating = true"
          class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors">
          <Plus class="w-3.5 h-3.5" />
          Create Token
        </button>

        <!-- Token list -->
        <div v-if="tokens.length > 0" class="border border-[var(--border)] rounded-lg overflow-hidden">
          <table class="w-full text-sm">
            <thead>
              <tr class="bg-[var(--bg-secondary)] text-[var(--text-secondary)] text-xs uppercase tracking-wider">
                <th class="text-left px-4 py-2 font-medium">Name</th>
                <th class="text-left px-4 py-2 font-medium">Token Prefix</th>
                <th class="text-left px-4 py-2 font-medium">Created</th>
                <th class="w-10 px-4 py-2"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-[var(--border)]">
              <tr v-for="t in tokens" :key="t.id" class="hover:bg-[var(--bg-hover)] transition-colors">
                <td class="px-4 py-2.5 text-[var(--text-primary)] font-medium">{{ t.name }}</td>
                <td class="px-4 py-2.5">
                  <code class="text-xs font-mono text-[var(--text-secondary)] bg-[var(--bg-primary)] px-1.5 py-0.5 rounded">{{ t.prefix }}...</code>
                </td>
                <td class="px-4 py-2.5 text-[var(--text-secondary)]">{{ formatDate(t.created_at) }}</td>
                <td class="px-4 py-2.5">
                  <button @click="confirmDelete = { id: t.id, name: t.name }"
                    class="p-1 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--danger)]">
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Empty state -->
        <div v-else-if="!creating" class="text-center text-[var(--text-secondary)] text-sm py-8">
          <p>No API tokens yet.</p>
          <p class="mt-1">Create a token to use the REST API from scripts and external tools.</p>
        </div>
      </div>
    </div>

    <!-- API Guide Tab -->
    <div v-else-if="activeTab === 'guide'" class="flex-1 overflow-y-auto">
      <div class="p-6 max-w-3xl space-y-8">

        <!-- Authentication -->
        <section>
          <h3 class="text-base font-semibold text-[var(--text-primary)] mb-3">Authentication</h3>
          <p class="text-sm text-[var(--text-secondary)] mb-3">
            Include your API token in the <code class="bg-[var(--bg-primary)] px-1 py-0.5 rounded text-xs">Authorization</code> header as a Bearer token:
          </p>
          <pre class="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg p-4 text-xs font-mono text-[var(--text-primary)] overflow-x-auto">curl -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  http://localhost:8080/api/v1/vms</pre>
        </section>

        <!-- Quick Examples -->
        <section>
          <h3 class="text-base font-semibold text-[var(--text-primary)] mb-3">Quick Examples</h3>
          <div class="space-y-4">

            <div>
              <h4 class="text-sm font-medium text-[var(--text-primary)] mb-1.5">List all VMs</h4>
              <pre class="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg p-3 text-xs font-mono text-[var(--text-primary)] overflow-x-auto">curl -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  http://localhost:8080/api/v1/vms</pre>
            </div>

            <div>
              <h4 class="text-sm font-medium text-[var(--text-primary)] mb-1.5">Get VM details</h4>
              <pre class="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg p-3 text-xs font-mono text-[var(--text-primary)] overflow-x-auto">curl -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  http://localhost:8080/api/v1/vms/my-vm</pre>
            </div>

            <div>
              <h4 class="text-sm font-medium text-[var(--text-primary)] mb-1.5">Start / Stop a VM</h4>
              <pre class="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg p-3 text-xs font-mono text-[var(--text-primary)] overflow-x-auto">curl -X POST -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  http://localhost:8080/api/v1/vms/my-vm/start

curl -X POST -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  http://localhost:8080/api/v1/vms/my-vm/stop</pre>
            </div>

            <div>
              <h4 class="text-sm font-medium text-[var(--text-primary)] mb-1.5">Create a VM</h4>
              <pre class="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg p-3 text-xs font-mono text-[var(--text-primary)] overflow-x-auto">curl -X POST -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{"name":"web-01","release":"24.04","cpus":2,"memory_mb":2048,"disk_gb":20}' \
  http://localhost:8080/api/v1/vms</pre>
            </div>

            <div>
              <h4 class="text-sm font-medium text-[var(--text-primary)] mb-1.5">List snapshots</h4>
              <pre class="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg p-3 text-xs font-mono text-[var(--text-primary)] overflow-x-auto">curl -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  http://localhost:8080/api/v1/vms/my-vm/snapshots</pre>
            </div>

            <div>
              <h4 class="text-sm font-medium text-[var(--text-primary)] mb-1.5">Execute a command in a VM</h4>
              <pre class="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg p-3 text-xs font-mono text-[var(--text-primary)] overflow-x-auto">curl -X POST -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{"command":"uname -a"}' \
  http://localhost:8080/api/v1/vms/my-vm/exec</pre>
            </div>

          </div>
        </section>

        <!-- Endpoint Reference -->
        <section>
          <h3 class="text-base font-semibold text-[var(--text-primary)] mb-3">Endpoint Reference</h3>
          <p class="text-sm text-[var(--text-secondary)] mb-4">All endpoints are prefixed with <code class="bg-[var(--bg-primary)] px-1 py-0.5 rounded text-xs">/api/v1</code>.</p>

          <div class="space-y-6">

            <!-- VMs -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">Virtual Machines</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List all VMs</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Get VM details</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create VM (async)</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/start</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Start VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/stop</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Stop VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/suspend</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Suspend VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Delete VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/recover</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Recover deleted VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/clone</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Clone VM from snapshot</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/exec</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Execute command in VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/start-all</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Start all VMs</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/stop-all</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Stop all VMs</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/purge</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Purge all deleted VMs</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/config</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Get VM CPU/memory/disk config</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/config</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Resize VM resources</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/cloud-init/status</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Cloud-init execution status</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Launches -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">Launch Tracking</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/launches</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List in-progress and failed launches</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/launches/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Dismiss launch entry</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Snapshots -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">Snapshots</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/snapshots</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List snapshots</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/snapshots</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create snapshot</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/snapshots/{snap}/restore</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Restore snapshot</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/snapshots/{snap}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Delete snapshot</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- File Transfer -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">File Transfer & Mounts</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/files/ls?path=...</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List files in VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/files?path=...</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Download file from VM</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/files</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Upload file to VM (multipart)</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/mounts</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List mounts</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/mounts</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Add mount</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/vms/{name}/mounts</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Remove mount</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Cloud-Init -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">Cloud-Init Templates</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/cloud-init/templates</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List all templates</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/cloud-init/templates/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Get template content</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/cloud-init/templates</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create template</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/cloud-init/templates/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Update template</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/cloud-init/templates/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Delete template</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Groups -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">VM Groups</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/groups</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List groups and members</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/groups</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create group</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/groups/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Rename group</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/groups/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Delete group</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/groups/assign</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Assign VM to group</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/groups/reorder</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Reorder groups</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Profiles -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">Launch Profiles</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/profiles</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List profiles</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/profiles</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create profile</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/profiles/{id}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Update profile</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/profiles/{id}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Delete profile</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Schedules -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">Scheduled Operations</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/schedules</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List schedules</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/schedules</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create schedule</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/schedules/{id}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Update schedule</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/schedules/{id}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Delete schedule</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/schedules/{id}/run</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Run schedule now</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/schedules/history</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Execution history</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Ansible -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">Ansible</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/status</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Ansible install status</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/inventory</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Generate dynamic inventory</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/playbooks</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List playbooks</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/playbooks/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Get playbook content</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/playbooks</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create playbook</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/playbooks/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Update playbook</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/playbooks/{name}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Delete playbook</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/run</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Run playbook (SSE stream)</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/run/status</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Current run status</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/run</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Cancel running playbook</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/ansible/run/queue</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Get run queue</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- System -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">System</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/version</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Server version and info</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/host/resources</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Host CPU, memory, disk</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/images</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Available Ubuntu images</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/networks</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Available network interfaces</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/config/vm-defaults</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Default VM creation settings</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-yellow-400">PUT</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/config/vm-defaults</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Update VM defaults</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/config/export</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Export configuration bundle</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/config/import</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Import configuration bundle</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- API Tokens -->
            <div>
              <h4 class="text-xs font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-2">API Tokens</h4>
              <div class="border border-[var(--border)] rounded-lg overflow-hidden">
                <table class="w-full text-xs">
                  <tbody class="divide-y divide-[var(--border)]">
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-green-400 w-16">GET</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/tokens</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">List API tokens</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-blue-400">POST</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/tokens</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Create token</td></tr>
                    <tr class="hover:bg-[var(--bg-hover)]"><td class="px-3 py-1.5 font-mono text-red-400">DELETE</td><td class="px-3 py-1.5 font-mono text-[var(--text-primary)]">/tokens/{id}</td><td class="px-3 py-1.5 text-[var(--text-secondary)]">Revoke token</td></tr>
                  </tbody>
                </table>
              </div>
            </div>

          </div>
        </section>

        <!-- Postman Collection -->
        <section class="border-t border-[var(--border)] pt-6">
          <h3 class="text-base font-semibold text-[var(--text-primary)] mb-3">Postman Collection</h3>
          <p class="text-sm text-[var(--text-secondary)] mb-3">
            Import this collection into Postman to get pre-configured requests for all endpoints. Set the <code class="bg-[var(--bg-primary)] px-1 py-0.5 rounded text-xs">token</code> and <code class="bg-[var(--bg-primary)] px-1 py-0.5 rounded text-xs">base_url</code> variables after importing.
          </p>
          <a
            href="/passgo-postman-collection.json"
            download="passgo-postman-collection.json"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors text-white"
          >
            <Download class="w-3.5 h-3.5" />
            Download Collection
          </a>
        </section>

      </div>
    </div>

    <!-- Delete confirmation -->
    <ConfirmModal
      v-if="confirmDelete"
      :message="`Delete token '${confirmDelete.name}'? Any scripts or integrations using this token will stop working immediately.`"
      @confirm="doDelete"
      @cancel="confirmDelete = null"
    />
  </div>
</template>
