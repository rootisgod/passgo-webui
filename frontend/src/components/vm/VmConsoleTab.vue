<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import { startVM, createShellSession, listShellSessions, deleteShellSession } from '../../api/client.js'
import { PowerOff, Play, Plus, X } from 'lucide-vue-next'
import ConsoleTerminal from './ConsoleTerminal.vue'

const store = useVmStore()
const toasts = useToastStore()
const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')
const isDeleted = computed(() => vm.value?.state === 'Deleted')
const starting = ref(false)

const consoleTabs = ref([])
const activeSessionId = ref(null)
let tabCounter = 0

async function powerOn() {
  starting.value = true
  try {
    await startVM(store.selectedNode)
    toasts.success(`${store.selectedNode} starting...`)
    store.fetchVMs()
  } catch (e) {
    toasts.error(e.message)
  } finally {
    starting.value = false
  }
}

async function initSessions() {
  const vmName = store.selectedNode
  try {
    const sessions = await listShellSessions(vmName)
    if (sessions.length > 0) {
      consoleTabs.value = sessions
        .filter(s => s.alive)
        .map(s => ({ sessionId: s.sessionId, label: `Shell ${++tabCounter}` }))
      if (consoleTabs.value.length > 0) {
        activeSessionId.value = consoleTabs.value[0].sessionId
        return
      }
    }
  } catch {
    // No existing sessions — will create one below
  }
  await addTab()
}

async function addTab() {
  const vmName = store.selectedNode
  try {
    const { sessionId } = await createShellSession(vmName)
    const tab = { sessionId, label: `Shell ${++tabCounter}` }
    consoleTabs.value.push(tab)
    activeSessionId.value = sessionId
  } catch (e) {
    toasts.error('Failed to create shell: ' + e.message)
  }
}

async function closeTab(sessionId) {
  const vmName = store.selectedNode
  try {
    await deleteShellSession(vmName, sessionId)
  } catch {
    // Session may already be dead — remove tab anyway
  }

  const idx = consoleTabs.value.findIndex(t => t.sessionId === sessionId)
  consoleTabs.value.splice(idx, 1)

  if (consoleTabs.value.length === 0) {
    // Last tab closed — create a replacement
    await addTab()
  } else if (activeSessionId.value === sessionId) {
    // Switch to nearest tab
    const newIdx = Math.min(idx, consoleTabs.value.length - 1)
    activeSessionId.value = consoleTabs.value[newIdx].sessionId
  }
}

function switchTab(sessionId) {
  activeSessionId.value = sessionId
}

watch(isRunning, (running) => {
  if (running && consoleTabs.value.length === 0) {
    initSessions()
  }
})

onMounted(() => {
  if (isRunning.value) {
    initSessions()
  }
})
</script>

<template>
  <!-- VM deleted -->
  <div v-if="isDeleted" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
    <PowerOff class="w-12 h-12 text-[var(--muted)]" />
    <p class="text-lg">VM Deleted</p>
    <p class="text-sm">Recover this VM to access the console</p>
  </div>

  <!-- VM not running -->
  <div v-else-if="!isRunning" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
    <PowerOff class="w-12 h-12 text-[var(--muted)]" />
    <p class="text-lg">Powered Off</p>
    <p class="text-sm">Start the VM to access the console</p>
    <button
      @click="powerOn"
      :disabled="starting"
      class="flex items-center gap-2 mt-2 px-4 py-2 text-sm rounded bg-green-900/30 hover:bg-[var(--success)] text-green-300 hover:text-white transition-colors disabled:opacity-40"
    >
      <Play class="w-4 h-4" />
      {{ starting ? 'Starting...' : 'Start VM' }}
    </button>
  </div>

  <!-- VM running — show tabbed console -->
  <div v-else class="flex flex-col h-full">
    <!-- Tab bar -->
    <div class="flex items-center bg-[var(--bg-surface)] border-b border-[var(--border)] overflow-x-auto">
      <button
        v-for="tab in consoleTabs"
        :key="tab.sessionId"
        @click="switchTab(tab.sessionId)"
        class="group flex items-center gap-1.5 px-3 py-1.5 text-xs border-r border-[var(--border)] whitespace-nowrap transition-colors"
        :class="tab.sessionId === activeSessionId
          ? 'bg-[var(--bg-primary)] text-[var(--text-primary)]'
          : 'bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
      >
        <span>{{ tab.label }}</span>
        <X
          @click.stop="closeTab(tab.sessionId)"
          class="w-3 h-3 opacity-0 group-hover:opacity-60 hover:!opacity-100 transition-opacity cursor-pointer"
        />
      </button>
      <button
        @click="addTab"
        class="flex items-center px-2 py-1.5 text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] transition-colors"
        title="New shell"
      >
        <Plus class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- Terminal panes (all mounted, only active visible) -->
    <div class="flex-1 relative">
      <ConsoleTerminal
        v-for="tab in consoleTabs"
        :key="tab.sessionId"
        :vmName="store.selectedNode"
        :sessionId="tab.sessionId"
        :active="tab.sessionId === activeSessionId"
        class="absolute inset-0"
        :class="{ 'invisible': tab.sessionId !== activeSessionId }"
      />
    </div>
  </div>
</template>
