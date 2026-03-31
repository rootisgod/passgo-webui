<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import { useWebSocket } from '../../composables/useWebSocket.js'
import { startVM } from '../../api/client.js'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { RefreshCw, PowerOff, Play } from 'lucide-vue-next'
import '@xterm/xterm/css/xterm.css'

const store = useVmStore()
const termRef = ref(null)
let term = null
let fitAddon = null
const { connected, error, connect, send, sendResize, disconnect } = useWebSocket()

const toasts = useToastStore()
const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')
const starting = ref(false)

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

const termInitialized = ref(false)

function initTerminal() {
  if (termInitialized.value || !termRef.value) return

  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    theme: {
      background: '#1a1a2e',
      foreground: '#e2e8f0',
      cursor: '#3b82f6',
      selectionBackground: '#3b82f640',
    },
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(termRef.value)

  nextTick(() => {
    fitAddon.fit()
  })

  term.onData((data) => {
    send(new TextEncoder().encode(data))
  })

  term.onResize(({ cols, rows }) => {
    sendResize(cols, rows)
  })

  connect(store.selectedNode, (data) => {
    term.write(data)
  })

  termInitialized.value = true
}

function reconnect() {
  if (term) {
    term.clear()
  }
  connect(store.selectedNode, (data) => {
    term.write(data)
  })
}

let resizeObserver = null

function setupResizeObserver() {
  if (resizeObserver) return
  resizeObserver = new ResizeObserver(() => {
    if (fitAddon) fitAddon.fit()
  })
  if (termRef.value) {
    resizeObserver.observe(termRef.value)
  }
}

// Watch for the VM becoming Running (e.g. after clicking Start VM)
watch(isRunning, (running) => {
  if (running && !termInitialized.value) {
    // Wait for the v-if to render the terminal div
    nextTick(() => {
      initTerminal()
      setupResizeObserver()
    })
  }
})

onMounted(() => {
  if (isRunning.value) {
    initTerminal()
    setupResizeObserver()
  }
})

onUnmounted(() => {
  disconnect()
  if (resizeObserver) resizeObserver.disconnect()
  if (term) term.dispose()
})
</script>

<template>
  <!-- VM not running -->
  <div v-if="!isRunning" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
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

  <!-- VM running — show terminal -->
  <div v-else class="flex flex-col h-full">
    <!-- Toolbar -->
    <div class="flex items-center gap-3 px-4 py-2 bg-[var(--bg-surface)] border-b border-[var(--border)]">
      <div class="flex items-center gap-2 text-sm">
        <span
          class="w-2 h-2 rounded-full"
          :class="connected ? 'bg-[var(--success)]' : 'bg-[var(--danger)]'"
        />
        <span class="text-[var(--text-secondary)]">
          {{ connected ? 'Connected' : error || 'Disconnected' }}
        </span>
      </div>
      <button
        v-if="!connected"
        @click="reconnect"
        class="flex items-center gap-1.5 px-2 py-1 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--accent)] transition-colors"
      >
        <RefreshCw class="w-3 h-3" />
        Reconnect
      </button>
    </div>

    <!-- Terminal -->
    <div ref="termRef" class="flex-1 p-1" />
  </div>
</template>
