<script setup>
import { ref, computed, onBeforeUnmount, watch } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import { startVM } from '../../api/client.js'
import { Monitor, Play, PowerOff, RefreshCw, WifiOff } from 'lucide-vue-next'

const props = defineProps({
  active: { type: Boolean, default: true },
})

const store = useVmStore()
const toasts = useToastStore()
const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')
const isDeleted = computed(() => vm.value?.state === 'Deleted')
const isTemplate = computed(() => vm.value ? store.isTemplate(vm.value.name) : false)

const container = ref(null)
const port = ref(5900)
const password = ref('')
const viewOnly = ref(false)
const starting = ref(false)
const status = ref('idle')
const errorMsg = ref('')

let rfb = null
let RFBClass = null

async function loadRFB() {
  if (!RFBClass) {
    const mod = await import('@novnc/novnc/lib/rfb.js')
    RFBClass = mod.default
  }
  return RFBClass
}

function wsURL() {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const params = new URLSearchParams({ port: String(port.value || 5900) })
  return `${proto}//${window.location.host}/api/v1/vms/${encodeURIComponent(store.selectedNode)}/vnc?${params}`
}

function disconnect(nextStatus = 'idle') {
  if (rfb) {
    try { rfb.disconnect() } catch { /* ignore */ }
    rfb = null
  }
  status.value = nextStatus
}

async function connect() {
  if (!container.value || !isRunning.value) return
  disconnect('idle')
  errorMsg.value = ''
  status.value = 'connecting'
  try {
    const RFB = await loadRFB()
    rfb = new RFB(container.value, wsURL(), {
      credentials: { password: password.value || '' },
      wsProtocols: ['binary'],
    })
    rfb.viewOnly = viewOnly.value
    rfb.scaleViewport = true
    rfb.resizeSession = false
    rfb.focusOnClick = true
    rfb.background = '#000000'

    rfb.addEventListener('connect', () => { status.value = 'connected' })
    rfb.addEventListener('disconnect', (event) => {
      rfb = null
      status.value = event.detail?.clean ? 'disconnected' : 'error'
      if (!event.detail?.clean && !errorMsg.value) {
        errorMsg.value = 'Disconnected unexpectedly'
      }
    })
    rfb.addEventListener('securityfailure', (event) => {
      status.value = 'error'
      errorMsg.value = event.detail?.reason || 'VNC security negotiation failed'
    })
    rfb.addEventListener('credentialsrequired', () => {
      status.value = 'error'
      errorMsg.value = 'VNC password required'
    })
  } catch (e) {
    status.value = 'error'
    errorMsg.value = e.message || 'failed to start VNC client'
  }
}

async function reconnect() {
  await connect()
}

async function powerOn() {
  if (isTemplate.value) return
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

watch(() => [props.active, store.selectedNode, isRunning.value], ([active, node, running]) => {
  if (!active || !node || !running) {
    disconnect('idle')
  }
})

watch(viewOnly, (value) => {
  if (rfb) rfb.viewOnly = value
})

onBeforeUnmount(() => {
  disconnect('idle')
})
</script>

<template>
  <div v-if="isDeleted" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
    <PowerOff class="w-12 h-12 text-[var(--muted)]" />
    <p class="text-lg">VM Deleted</p>
    <p class="text-sm">Recover this VM to access graphics</p>
  </div>

  <div v-else-if="!isRunning" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
    <PowerOff class="w-12 h-12 text-[var(--muted)]" />
    <p class="text-lg">Powered Off</p>
    <p class="text-sm">{{ isTemplate ? 'Remove the template tag to start this VM' : 'Start the VM to access graphics' }}</p>
    <button
      @click="powerOn"
      :disabled="starting || isTemplate"
      class="flex items-center gap-2 mt-2 px-4 py-2 text-sm rounded bg-green-900/30 hover:bg-[var(--success)] text-green-300 hover:text-white transition-colors disabled:opacity-40"
    >
      <Play class="w-4 h-4" />
      {{ isTemplate ? 'Template Protected' : starting ? 'Starting...' : 'Start VM' }}
    </button>
  </div>

  <div v-else class="flex flex-col h-full bg-black">
    <div class="flex flex-wrap items-center justify-between gap-3 px-3 py-2 bg-[var(--bg-surface)] border-b border-[var(--border)] text-xs">
      <div class="flex items-center gap-2 text-[var(--text-secondary)] min-w-0">
        <Monitor class="w-3.5 h-3.5 flex-shrink-0" />
        <span v-if="status === 'connecting'">Connecting...</span>
        <span v-else-if="status === 'connected'" class="text-[var(--success)]">Connected</span>
        <span v-else-if="status === 'disconnected'">Disconnected</span>
        <span v-else-if="status === 'error'" class="text-red-400 truncate">{{ errorMsg || 'Connection failed' }}</span>
        <span v-else>Idle</span>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <label class="flex items-center gap-1.5 text-[var(--text-secondary)]">
          <span>Port</span>
          <input
            v-model.number="port"
            type="number"
            min="5900"
            max="5999"
            class="w-20 bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            :disabled="status === 'connecting' || status === 'connected'"
          />
        </label>
        <label class="flex items-center gap-1.5 text-[var(--text-secondary)]">
          <span>Password</span>
          <input
            v-model="password"
            type="password"
            autocomplete="off"
            class="w-28 bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            :disabled="status === 'connecting' || status === 'connected'"
          />
        </label>
        <label class="flex items-center gap-1.5 text-[var(--text-secondary)]">
          <input
            v-model="viewOnly"
            type="checkbox"
            class="w-3.5 h-3.5 rounded border-[var(--border)] bg-[var(--bg-primary)] text-[var(--accent)] focus:ring-0 focus:ring-offset-0"
          />
          <span>View only</span>
        </label>
        <button
          v-if="status !== 'connecting' && status !== 'connected'"
          @click="reconnect"
          class="flex items-center gap-1 px-2 py-1 rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] text-[var(--text-secondary)] transition-colors"
        >
          <RefreshCw class="w-3 h-3" />
          Connect
        </button>
        <button
          v-else
          @click="disconnect('disconnected')"
          class="flex items-center gap-1 px-2 py-1 rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] text-[var(--text-secondary)] transition-colors"
        >
          <WifiOff class="w-3 h-3" />
          Disconnect
        </button>
      </div>
    </div>

    <div ref="container" class="flex-1 relative overflow-hidden">
      <div
        v-if="status === 'connecting'"
        class="absolute inset-0 flex items-center justify-center text-[var(--text-secondary)] text-sm pointer-events-none"
      >
        Connecting to VNC...
      </div>
      <div
        v-else-if="status === 'idle' || status === 'disconnected'"
        class="absolute inset-0 flex items-center justify-center text-[var(--text-secondary)] text-sm pointer-events-none"
      >
        Not connected
      </div>
    </div>
  </div>
</template>
