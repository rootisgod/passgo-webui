<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { getVncConsole } from '../../api/client.js'
import { Monitor, PowerOff, RefreshCw } from 'lucide-vue-next'

const store = useVmStore()
const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')
const loading = ref(false)
const consoleInfo = ref(null)
const error = ref(null)
const iframeKey = ref(0)

async function loadConsole() {
  if (!store.selectedNode || !isRunning.value) return
  loading.value = true
  error.value = null
  try {
    consoleInfo.value = await getVncConsole(store.selectedNode)
  } catch (e) {
    error.value = e.message
    consoleInfo.value = null
  } finally {
    loading.value = false
  }
}

function refreshFrame() {
  iframeKey.value += 1
  loadConsole()
}

watch(() => store.selectedNode, () => {
  consoleInfo.value = null
  error.value = null
  iframeKey.value += 1
  if (isRunning.value) loadConsole()
})

watch(isRunning, (running) => {
  consoleInfo.value = null
  error.value = null
  iframeKey.value += 1
  if (running) loadConsole()
})

onMounted(() => {
  if (isRunning.value) loadConsole()
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
        <span>VNC / noVNC console</span>
        <span v-if="consoleInfo?.host" class="text-xs text-[var(--muted)]">
          {{ consoleInfo.host }}:{{ consoleInfo.port }}
        </span>
      </div>
      <button
        @click="refreshFrame"
        :disabled="loading"
        class="flex items-center gap-1.5 px-2 py-1 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--accent)] transition-colors disabled:opacity-40"
      >
        <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': loading }" />
        Refresh
      </button>
    </div>

    <div v-if="loading && !consoleInfo" class="flex-1 flex items-center justify-center text-[var(--text-secondary)]">
      Checking noVNC console...
    </div>

    <div v-else-if="error" class="flex-1 flex flex-col items-center justify-center gap-3 p-6 text-center text-[var(--text-secondary)]">
      <Monitor class="w-12 h-12 text-[var(--muted)]" />
      <p class="text-lg text-[var(--text-primary)]">Unable to load VNC console</p>
      <p class="max-w-2xl text-sm">{{ error }}</p>
    </div>

    <div v-else-if="consoleInfo && !consoleInfo.available" class="flex-1 flex flex-col items-center justify-center gap-3 p-6 text-center text-[var(--text-secondary)]">
      <Monitor class="w-12 h-12 text-[var(--muted)]" />
      <p class="text-lg text-[var(--text-primary)]">No noVNC service detected</p>
      <p class="max-w-2xl text-sm">{{ consoleInfo.message }}</p>
      <p class="max-w-2xl text-xs text-[var(--muted)]">
        Expected a noVNC web service inside the guest on port 6080. For Ubuntu agent desktops, add a cloud-init/profile step that installs a desktop, VNC server, and noVNC/websockify.
      </p>
    </div>

    <iframe
      v-else-if="consoleInfo?.url"
      :key="iframeKey"
      :src="consoleInfo.url"
      class="flex-1 w-full border-0 bg-black"
      title="VNC console"
      allow="clipboard-read; clipboard-write"
    />
  </div>
</template>
