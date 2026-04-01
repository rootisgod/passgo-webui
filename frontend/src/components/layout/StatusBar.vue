<script setup>
import { ref, computed, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { getVersion } from '../../api/client.js'
import { Wifi, WifiOff } from 'lucide-vue-next'

const store = useVmStore()
const buildTime = ref('')

onMounted(async () => {
  try {
    const data = await getVersion()
    buildTime.value = data.build_time || 'unknown'
  } catch {
    buildTime.value = 'unknown'
  }
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
      <span>{{ store.totalCount }} VMs</span>
      <span>Last refresh: {{ timeSinceRefresh }}</span>
    </div>
  </footer>
</template>
