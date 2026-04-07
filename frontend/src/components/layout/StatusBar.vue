<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { getVersion } from '../../api/client.js'
import { Wifi, WifiOff } from 'lucide-vue-next'

const store = useVmStore()
const buildTime = ref('')
const serverTime = ref('')
const serverTimezone = ref('')

// Calculate offset between server and local clocks so we can tick locally
let serverOffset = 0
let clockInterval = null

function updateClock() {
  const now = new Date(Date.now() + serverOffset)
  serverTime.value = now.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

onMounted(async () => {
  try {
    const data = await getVersion()
    buildTime.value = data.build_time || 'unknown'
    if (data.server_time) {
      const serverNow = new Date(data.server_time)
      serverOffset = serverNow.getTime() - Date.now()
      serverTimezone.value = data.timezone || ''
      updateClock()
      clockInterval = setInterval(updateClock, 1000)
    }
  } catch {
    buildTime.value = 'unknown'
  }
})

onUnmounted(() => {
  if (clockInterval) clearInterval(clockInterval)
})

const timeSinceRefresh = computed(() => {
  if (!store.lastRefresh) return 'never'
  const secs = Math.floor((Date.now() - store.lastRefresh.getTime()) / 1000)
  if (secs < 5) return 'just now'
  return `${secs}s ago`
})
</script>

<template>
  <footer class="flex items-center justify-between px-4 py-1.5 bg-[var(--bg-secondary)] border-t border-[var(--border)] text-xs text-[var(--text-secondary)]">
    <div class="flex items-center gap-2">
      <Wifi v-if="!store.error" class="w-3 h-3 text-[var(--success)]" />
      <WifiOff v-else class="w-3 h-3 text-[var(--danger)]" />
      <span>{{ store.error ? 'Disconnected' : 'Connected' }}</span>
    </div>
    <div v-if="buildTime" class="text-[var(--text-muted)]">
      Built: {{ buildTime }}
    </div>
    <div class="flex items-center gap-4">
      <span v-if="serverTime" class="font-mono">{{ serverTime }}<span v-if="serverTimezone" class="ml-1 text-[var(--text-muted)]">{{ serverTimezone }}</span></span>
      <span>{{ store.totalCount }} VMs</span>
      <span>Last refresh: {{ timeSinceRefresh }}</span>
    </div>
  </footer>
</template>
