<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { getVncConsole } from '../../api/client.js'
import { Monitor, PowerOff, RefreshCw } from 'lucide-vue-next'

const store = useVmStore()
const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')
const screenRef = ref(null)
const loading = ref(false)
const connected = ref(false)
const consoleInfo = ref(null)
const error = ref(null)

let rfb = null
let generation = 0

function disconnectConsole() {
  if (rfb) {
    rfb.disconnect()
    rfb = null
  }
  connected.value = false
}

function websocketURL(path) {
  const url = new URL(path, window.location.href)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.href
}

async function connectConsole(info, requestGeneration) {
  await nextTick()
  if (requestGeneration !== generation || !screenRef.value) return

  const { default: RFB } = await import('@novnc/novnc')
  if (requestGeneration !== generation || !screenRef.value) return

  rfb = new RFB(screenRef.value, websocketURL(info.url), {
    credentials: { password: info.password },
  })
  rfb.scaleViewport = true
  rfb.resizeSession = true
  rfb.background = '#000000'

  rfb.addEventListener('connect', () => {
    if (requestGeneration === generation) connected.value = true
  })
  rfb.addEventListener('credentialsrequired', () => {
    rfb?.sendCredentials({ password: info.password })
  })
  rfb.addEventListener('securityfailure', (event) => {
    if (requestGeneration === generation) {
      error.value = event.detail?.reason || 'VNC authentication failed'
    }
  })
  rfb.addEventListener('disconnect', (event) => {
    if (requestGeneration === generation) {
      connected.value = false
      if (!event.detail?.clean && !error.value) error.value = 'VNC connection closed unexpectedly'
    }
  })
}

async function loadConsole() {
  const vmName = store.selectedNode
  const requestGeneration = ++generation
  disconnectConsole()
  consoleInfo.value = null
  error.value = null
  if (!vmName || !isRunning.value) return

  loading.value = true
  try {
    const info = await getVncConsole(vmName)
    if (requestGeneration !== generation) return
    consoleInfo.value = info
    loading.value = false
    if (info.available && info.url) await connectConsole(info, requestGeneration)
  } catch (e) {
    if (requestGeneration === generation) error.value = e.message
  } finally {
    if (requestGeneration === generation) loading.value = false
  }
}

watch(() => store.selectedNode, loadConsole)
watch(isRunning, loadConsole)
onMounted(loadConsole)
onBeforeUnmount(() => {
  generation += 1
  disconnectConsole()
})
</script>

<template>
  <div v-if="!isRunning" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
    <PowerOff class="w-12 h-12 text-[var(--muted)]" />
    <p class="text-lg">VM Powered Off</p>
    <p class="text-sm">Start the VM to access the graphical console.</p>
  </div>

  <div v-else class="flex flex-col h-full">
    <div class="flex items-center justify-between gap-3 px-4 py-2 bg-[var(--bg-surface)] border-b border-[var(--border)]">
      <div class="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
        <Monitor class="w-4 h-4" />
        <span>VNC console</span>
        <span class="flex items-center gap-1 text-xs text-[var(--muted)]">
          <span class="w-2 h-2 rounded-full" :class="connected ? 'bg-[var(--success)]' : 'bg-[var(--danger)]'" />
          {{ connected ? 'Connected' : 'Disconnected' }}
        </span>
      </div>
      <button
        type="button"
        title="Reconnect VNC console"
        aria-label="Reconnect VNC console"
        @click="loadConsole"
        :disabled="loading"
        class="p-1.5 rounded bg-[var(--bg-hover)] hover:bg-[var(--accent)] transition-colors disabled:opacity-40"
      >
        <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
      </button>
    </div>

    <div v-if="loading" class="flex-1 flex items-center justify-center text-[var(--text-secondary)]">
      Connecting to VNC...
    </div>

    <div v-else-if="error" class="flex-1 flex flex-col items-center justify-center gap-3 p-6 text-center text-[var(--text-secondary)]">
      <Monitor class="w-12 h-12 text-[var(--muted)]" />
      <p class="text-lg text-[var(--text-primary)]">Unable to load VNC console</p>
      <p class="max-w-2xl text-sm">{{ error }}</p>
    </div>

    <div v-else-if="consoleInfo && !consoleInfo.available" class="flex-1 flex flex-col items-center justify-center gap-3 p-6 text-center text-[var(--text-secondary)]">
      <Monitor class="w-12 h-12 text-[var(--muted)]" />
      <p class="text-lg text-[var(--text-primary)]">VNC is not available</p>
      <p class="max-w-2xl text-sm">{{ consoleInfo.message }}</p>
    </div>

    <div v-else ref="screenRef" class="flex-1 min-h-0 overflow-hidden bg-black" />
  </div>
</template>
