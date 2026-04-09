<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import KeyboardShortcuts from '../shared/KeyboardShortcuts.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import PlaybookEditor from './PlaybookEditor.vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { Plus, Save, Trash2, Maximize2, Minimize2, WrapText } from 'lucide-vue-next'

const props = defineProps({
  vmName: { type: String, required: true },
})

const toasts = useToastStore()

// Ansible status
const ansibleInstalled = ref(false)
const ansibleVersion = ref('')
const statusLoading = ref(true)

// Playbook CRUD state
const playbooks = ref([])
const selectedPlaybook = ref(null)
const editorContent = ref('')
const originalContent = ref('')
const dirty = computed(() => editorContent.value !== originalContent.value)
const isNew = ref(false)
const newFileName = ref('')
const saving = ref(false)
const confirmAction = ref(null)
const editorFullscreen = ref(false)
const wordWrap = ref(false)

// Target is always the current VM
const targetVMs = computed(() => [props.vmName])

// Execution state
const isRunning = ref(false)
const runStatus = ref('idle') // idle | running | success | failed
let abortController = null

// Queue state
const ansibleQueue = ref([])
let queuePollInterval = null

// Terminal output
const termRef = ref(null)
let term = null
let fitAddon = null
let resizeObserver = null

function initTerminal() {
  if (term || !termRef.value) return
  term = new Terminal({
    cursorBlink: false,
    fontSize: 13,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    disableStdin: true,
    convertEol: true,
    theme: {
      background: '#1a1a2e',
      foreground: '#e2e8f0',
      cursor: '#1a1a2e',
      selectionBackground: '#3b82f640',
    },
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(termRef.value)
  fitAddon.fit()

  resizeObserver = new ResizeObserver(() => {
    if (fitAddon) fitAddon.fit()
  })
  resizeObserver.observe(termRef.value)
}

async function pollQueue() {
  try {
    const q = await api.getAnsibleRunQueue()
    ansibleQueue.value = Array.isArray(q) ? q : []
  } catch { ansibleQueue.value = [] }
}

onMounted(async () => {
  await checkStatus()
  await loadPlaybooks()
  await nextTick()
  initTerminal()
  await checkExistingRun()
  await pollQueue()
  queuePollInterval = setInterval(pollQueue, 5000)
})

onUnmounted(() => {
  if (resizeObserver) resizeObserver.disconnect()
  if (term) { term.dispose(); term = null }
  // Only abort the SSE stream, NOT the ansible process
  if (abortController) abortController.abort()
  if (queuePollInterval) clearInterval(queuePollInterval)
})

async function checkExistingRun() {
  try {
    const status = await api.getAnsibleRunStatus()
    if (!status.active) return
    // There's an active or completed run — reconnect to its output
    selectedPlaybook.value = status.playbook
    if (status.status === 'running') {
      isRunning.value = true
      runStatus.value = 'running'
      connectToOutput()
    } else {
      // Completed run — replay output
      runStatus.value = status.status === 'success' ? 'success' : 'failed'
      connectToOutput()
    }
  } catch { /* no active run */ }
}

// Connect to the SSE output stream — replays buffered output then streams live
function connectToOutput() {
  abortController = new AbortController()

  fetch('/api/v1/ansible/run/output', {
    signal: abortController.signal,
  }).then(async (response) => {
    if (!response.ok) return

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop()
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        try {
          const event = JSON.parse(line.slice(6))
          if (event.type === 'output') {
            if (term) term.writeln(event.content)
          } else if (event.type === 'done') {
            runStatus.value = event.exit_code === 0 ? 'success' : 'failed'
            isRunning.value = false
          } else if (event.type === 'error') {
            if (term) term.writeln(`\x1b[31m${event.content}\x1b[0m`)
            runStatus.value = 'failed'
            isRunning.value = false
          }
        } catch { /* skip malformed events */ }
      }
    }
  }).catch((e) => {
    if (e.name !== 'AbortError') {
      runStatus.value = 'failed'
      isRunning.value = false
    }
  })
}

async function checkStatus() {
  try {
    const status = await api.getAnsibleStatus()
    ansibleInstalled.value = status.installed
    ansibleVersion.value = status.version || ''
  } catch {
    ansibleInstalled.value = false
  } finally {
    statusLoading.value = false
  }
}

async function loadPlaybooks() {
  try {
    const list = await api.listPlaybooks()
    playbooks.value = Array.isArray(list) ? list : []
  } catch { playbooks.value = [] }
}

async function selectPlaybook(name) {
  if (dirty.value && !confirm('Discard unsaved changes?')) return
  try {
    const result = await api.getPlaybook(name)
    selectedPlaybook.value = name
    editorContent.value = result.content
    originalContent.value = result.content
    isNew.value = false
  } catch (e) { toasts.error(e.message) }
}

function newPlaybook() {
  if (dirty.value && !confirm('Discard unsaved changes?')) return
  selectedPlaybook.value = null
  editorContent.value = '---\n- hosts: all\n  become: true\n  tasks:\n    - name: Example task\n      ping:\n'
  originalContent.value = ''
  isNew.value = true
  newFileName.value = ''
}

async function save() {
  saving.value = true
  try {
    if (isNew.value) {
      let name = newFileName.value.trim()
      if (!name) { toasts.error('Filename is required'); return }
      if (!name.endsWith('.yml') && !name.endsWith('.yaml')) name += '.yml'
      await api.createPlaybook(name, editorContent.value)
      selectedPlaybook.value = name
      originalContent.value = editorContent.value
      isNew.value = false
      toasts.success(`Playbook ${name} created`)
      await loadPlaybooks()
    } else if (selectedPlaybook.value) {
      await api.updatePlaybook(selectedPlaybook.value, editorContent.value)
      originalContent.value = editorContent.value
      toasts.success('Playbook saved')
    }
  } catch (e) { toasts.error(e.message) }
  finally { saving.value = false }
}

function confirmDelete() {
  if (!selectedPlaybook.value) return
  confirmAction.value = {
    fn: doDelete,
    message: `Delete playbook '${selectedPlaybook.value}'?`,
  }
}

async function doDelete() {
  try {
    await api.deletePlaybook(selectedPlaybook.value)
    toasts.success('Playbook deleted')
    selectedPlaybook.value = null
    editorContent.value = ''
    originalContent.value = ''
    isNew.value = false
    await loadPlaybooks()
  } catch (e) { toasts.error(e.message) }
}

async function executeConfirmed() {
  const fn = confirmAction.value?.fn
  confirmAction.value = null
  if (fn) await fn()
}

const canRun = computed(() => selectedPlaybook.value && !isNew.value && !dirty.value && !isRunning.value && ansibleInstalled.value)

async function runPlaybook() {
  if (!canRun.value) return
  if (term) term.clear()
  // Clear any previous completed run
  try { await api.clearAnsibleRun() } catch { /* ignore */ }

  runStatus.value = 'running'
  isRunning.value = true

  try {
    const response = await fetch('/api/v1/ansible/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        playbook: selectedPlaybook.value,
        vms: targetVMs.value,
      }),
    })

    if (!response.ok) {
      const err = await response.json().catch(() => ({ error: 'Unknown error' }))
      if (term) term.writeln(`\x1b[31mError: ${err.error}\x1b[0m`)
      runStatus.value = 'failed'
      isRunning.value = false
      return
    }

    // Run started — connect to the output stream
    connectToOutput()
  } catch (e) {
    if (term) term.writeln(`\x1b[31mError: ${e.message}\x1b[0m`)
    runStatus.value = 'failed'
    isRunning.value = false
  }
}

async function cancelRun() {
  try {
    await api.cancelAnsibleRun()
  } catch (e) { toasts.error(e.message) }
}

async function clearOutput() {
  if (term) term.clear()
  runStatus.value = 'idle'
  try { await api.clearAnsibleRun() } catch { /* ignore */ }
}

async function clearQueue() {
  try {
    await api.clearAnsibleRunQueue()
    ansibleQueue.value = []
  } catch (e) { toasts.error(e.message) }
}

const statusColor = computed(() => {
  switch (runStatus.value) {
    case 'running': return 'text-[var(--accent)]'
    case 'success': return 'text-[var(--success)]'
    case 'failed': return 'text-[var(--danger)]'
    default: return 'text-[var(--muted)]'
  }
})

const statusLabel = computed(() => {
  switch (runStatus.value) {
    case 'running': return 'Running...'
    case 'success': return 'Success'
    case 'failed': return 'Failed'
    default: return 'Idle'
  }
})
</script>

<template>
  <div class="flex flex-col h-full p-4 gap-4" v-if="!statusLoading">
    <!-- Ansible not installed -->
    <div v-if="!ansibleInstalled" class="flex items-center justify-center h-full">
      <div class="text-center max-w-md">
        <h3 class="text-lg font-semibold mb-2 text-[var(--text-primary)]">Ansible Not Found</h3>
        <p class="text-sm text-[var(--text-secondary)] mb-4">
          <code>ansible-playbook</code> was not found in PATH. Install Ansible on the host machine to use this feature.
        </p>
        <div class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-4 text-left">
          <p class="text-xs text-[var(--text-secondary)] mb-2">Install with:</p>
          <code class="block text-xs font-mono text-[var(--text-primary)] mb-1">pip install ansible</code>
          <code class="block text-xs font-mono text-[var(--text-primary)] mb-1">brew install ansible</code>
          <code class="block text-xs font-mono text-[var(--text-primary)]">apt install ansible</code>
        </div>
      </div>
    </div>

    <template v-else>
      <!-- Toolbar -->
      <div class="flex items-center gap-2 flex-shrink-0">
        <ActionButton label="New" :icon="Plus" @click="newPlaybook" :disabled="isRunning" />
        <ActionButton label="Save" :icon="Save" variant="success" @click="save" :disabled="!dirty && !isNew || saving || isRunning" />
        <ActionButton label="Delete" :icon="Trash2" variant="danger" @click="confirmDelete" :disabled="!selectedPlaybook || isNew || isRunning" />
        <div class="flex-1" />
        <button
          v-if="selectedPlaybook || isNew"
          @click="wordWrap = !wordWrap"
          class="p-1.5 rounded hover:bg-[var(--bg-hover)] transition-colors"
          :class="wordWrap ? 'text-[var(--accent)]' : 'text-[var(--text-secondary)]'"
          title="Toggle word wrap"
        >
          <WrapText class="w-4 h-4" />
        </button>
        <button
          v-if="selectedPlaybook || isNew"
          @click="editorFullscreen = !editorFullscreen"
          class="p-1.5 rounded hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
          :title="editorFullscreen ? 'Exit fullscreen' : 'Fullscreen editor'"
        >
          <Minimize2 v-if="editorFullscreen" class="w-4 h-4" />
          <Maximize2 v-else class="w-4 h-4" />
        </button>
        <KeyboardShortcuts v-if="selectedPlaybook || isNew" />
        <span class="text-xs text-[var(--muted)]">{{ ansibleVersion }}</span>
      </div>

      <!-- Main content area -->
      <div class="flex gap-4 flex-1 min-h-0">
        <!-- Playbook list -->
        <div class="w-44 flex-shrink-0 bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] overflow-auto">
          <div class="p-2 text-xs text-[var(--text-secondary)] uppercase tracking-wider font-medium">Playbooks</div>
          <div v-if="playbooks.length === 0" class="px-3 py-2 text-xs text-[var(--muted)]">No playbooks yet</div>
          <button
            v-for="pb in playbooks"
            :key="pb.name"
            class="w-full text-left px-3 py-1.5 text-sm truncate transition-colors"
            :class="selectedPlaybook === pb.name
              ? 'bg-[var(--accent)]/20 text-[var(--accent)]'
              : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-primary)]'"
            @click="selectPlaybook(pb.name)"
          >
            {{ pb.name }}
          </button>
        </div>

        <!-- Editor -->
        <div class="flex-1 min-w-0 flex flex-col">
          <!-- New file name input -->
          <div v-if="isNew" class="mb-2">
            <input
              v-model="newFileName"
              type="text"
              placeholder="playbook-name.yml"
              class="w-full px-3 py-1.5 bg-[var(--bg-surface)] border border-[var(--border)] rounded text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
            />
          </div>
          <div class="flex-1 min-h-0" v-if="selectedPlaybook || isNew">
            <PlaybookEditor v-model="editorContent" :fullscreen="editorFullscreen" :word-wrap="wordWrap" @exit-fullscreen="editorFullscreen = false" />
          </div>
          <div v-else class="flex-1 flex items-center justify-center text-sm text-[var(--muted)]">
            Select a playbook or create a new one
          </div>
        </div>

        <!-- Run controls -->
        <div class="w-44 flex-shrink-0 bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] flex flex-col">
          <div class="p-2 text-xs text-[var(--text-secondary)] uppercase tracking-wider font-medium">Target</div>
          <div class="px-3 py-2 text-sm text-[var(--text-primary)] flex-1">{{ vmName }}</div>
          <div class="p-2 border-t border-[var(--border)]">
            <button
              v-if="!isRunning"
              class="w-full px-3 py-2 rounded text-sm font-medium transition-colors"
              :class="canRun
                ? 'bg-[var(--accent)] text-white hover:opacity-90'
                : 'bg-[var(--bg-primary)] text-[var(--muted)] cursor-not-allowed'"
              :disabled="!canRun"
              @click="runPlaybook"
            >
              Run Playbook
            </button>
            <button
              v-else
              class="w-full px-3 py-2 rounded text-sm font-medium bg-[var(--danger)] text-white hover:opacity-90 transition-colors"
              @click="cancelRun"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>

      <!-- Queue indicator -->
      <div v-if="ansibleQueue.length > 0" class="flex-shrink-0 px-3 py-2 bg-blue-900/20 border border-blue-800/30 rounded-lg">
        <div class="flex items-center justify-between mb-1">
          <span class="text-xs text-[var(--accent)] font-medium">Queued Runs ({{ ansibleQueue.length }})</span>
          <button @click="clearQueue" class="text-xs text-[var(--muted)] hover:text-[var(--text-primary)] transition-colors">Clear Queue</button>
        </div>
        <div v-for="(entry, i) in ansibleQueue" :key="i" class="text-xs text-[var(--text-secondary)]">
          {{ entry.playbook }} &rarr; {{ entry.vms.join(', ') }}
        </div>
      </div>

      <!-- Output panel -->
      <div class="flex-shrink-0 bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] flex flex-col" style="height: 280px;">
        <div class="flex items-center justify-between px-3 py-1.5 border-b border-[var(--border)]">
          <div class="flex items-center gap-2">
            <span class="text-xs text-[var(--text-secondary)] uppercase tracking-wider font-medium">Output</span>
            <span class="text-xs" :class="statusColor">{{ statusLabel }}</span>
          </div>
          <button
            class="text-xs text-[var(--muted)] hover:text-[var(--text-primary)] transition-colors"
            @click="clearOutput"
            :disabled="isRunning"
          >
            Clear
          </button>
        </div>
        <div ref="termRef" class="flex-1 min-h-0" />
      </div>
    </template>

    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />
  </div>
</template>
