<script setup>
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useWebSocket } from '../../composables/useWebSocket.js'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { RefreshCw } from 'lucide-vue-next'
import '@xterm/xterm/css/xterm.css'

const props = defineProps({
  vmName: { type: String, required: true },
  sessionId: { type: String, required: true },
  active: { type: Boolean, default: true },
})

const termRef = ref(null)
let term = null
let fitAddon = null
let resizeObserver = null
const { connected, error, connect, send, sendResize, disconnect } = useWebSocket()

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

  connect(props.vmName, props.sessionId, (data) => {
    term.write(data)
  })

  termInitialized.value = true
  setupResizeObserver()
}

function reconnect() {
  connect(props.vmName, props.sessionId, (data) => {
    term.write(data)
  })
}

function setupResizeObserver() {
  if (resizeObserver) return
  resizeObserver = new ResizeObserver(() => {
    if (fitAddon && props.active) fitAddon.fit()
  })
  if (termRef.value) {
    resizeObserver.observe(termRef.value)
  }
}

// When this tab becomes active, refit the terminal (xterm needs remeasure after becoming visible)
watch(() => props.active, (isActive) => {
  if (isActive && fitAddon) {
    nextTick(() => fitAddon.fit())
  }
})

onMounted(() => {
  initTerminal()
})

onUnmounted(() => {
  disconnect()
  if (resizeObserver) resizeObserver.disconnect()
  if (term) term.dispose()
})
</script>

<template>
  <div class="flex flex-col h-full">
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
