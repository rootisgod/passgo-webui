<script setup>
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import * as api from '../../api/client.js'
import { Cloud, ChevronDown, ChevronRight, Loader2, CheckCircle, AlertCircle, XCircle } from 'lucide-vue-next'

const props = defineProps({
  vmName: { type: String, required: true },
})

const status = ref(null)
const loading = ref(true)
const expanded = ref(false)
const error = ref(null)
const logEl = ref(null)
let timer = null
let stopped = false

// Normalize status strings -- cloud-init can return compound states like "degraded running"
const normalizedStatus = computed(() => {
  const s = status.value?.status
  if (!s) return null
  if (s.includes('running')) return 'running'
  if (s.includes('error') || s.includes('degraded')) return 'degraded'
  return s
})

const statusIcon = computed(() => {
  switch (normalizedStatus.value) {
    case 'done': return CheckCircle
    case 'degraded': return AlertCircle
    case 'error': return XCircle
    case 'running': case 'pending': case null: return Loader2
    default: return Cloud
  }
})

const statusColor = computed(() => {
  switch (normalizedStatus.value) {
    case 'done': return 'text-[var(--success)]'
    case 'error': return 'text-[var(--danger)]'
    case 'running': case 'pending': case null: return 'text-[var(--accent)]'
    case 'degraded': return 'text-[var(--warning)]'
    default: return 'text-[var(--text-secondary)]'
  }
})

const statusLabel = computed(() => {
  const s = status.value?.status
  if (!s) return 'Checking...'
  switch (normalizedStatus.value) {
    case 'done': return 'Complete'
    case 'error': return 'Failed'
    case 'running': return 'Running...'
    case 'pending': return 'Waiting for VM...'
    case 'degraded': return 'Complete (with warnings)'
    default: return s
  }
})

const isActive = computed(() => {
  const s = normalizedStatus.value
  return s === null || s === 'running' || s === 'pending'
})

// Auto-scroll log to bottom when output updates
watch(() => status.value?.output, async () => {
  if (expanded.value && logEl.value) {
    await nextTick()
    logEl.value.scrollTop = logEl.value.scrollHeight
  }
})

// Auto-expand when cloud-init is actively running
watch(isActive, (active) => {
  if (active && !expanded.value && status.value?.output) {
    expanded.value = true
  }
}, { immediate: true })

async function fetchStatus() {
  if (stopped) return
  try {
    error.value = null
    status.value = await api.getCloudInitStatus(props.vmName)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }

  // Keep polling while active
  if (isActive.value && !stopped) {
    timer = setTimeout(fetchStatus, 4000)
  }
}

onMounted(() => {
  fetchStatus()
})

onUnmounted(() => {
  stopped = true
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
})
</script>

<template>
  <div class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] overflow-hidden">
    <!-- Header -->
    <button
      @click="expanded = !expanded"
      class="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-[var(--bg-hover)] transition-colors"
    >
      <Cloud class="w-4 h-4 text-[var(--text-secondary)]" />
      <span class="text-sm font-medium">Cloud-Init</span>

      <!-- Status badge -->
      <div class="flex items-center gap-1.5" :class="statusColor">
        <component
          :is="statusIcon"
          class="w-3.5 h-3.5"
          :class="{ 'animate-spin': isActive || loading }"
        />
        <span class="text-xs">{{ statusLabel }}</span>
      </div>

      <div v-if="status?.detail && isActive" class="text-xs text-[var(--text-secondary)] truncate flex-1">
        {{ status.detail }}
      </div>

      <div class="flex-1" />

      <ChevronDown v-if="expanded" class="w-4 h-4 text-[var(--text-secondary)]" />
      <ChevronRight v-else class="w-4 h-4 text-[var(--text-secondary)]" />
    </button>

    <!-- Expanded content -->
    <div v-if="expanded" class="border-t border-[var(--border)]">
      <!-- Errors -->
      <div v-if="status?.errors?.length" class="px-4 py-2 bg-red-900/10 border-b border-[var(--border)]">
        <div class="text-xs font-medium text-[var(--danger)] mb-1">Errors</div>
        <div v-for="(err, i) in status.errors" :key="i" class="text-xs text-[var(--danger)] font-mono">{{ err }}</div>
      </div>

      <!-- Output log -->
      <div class="px-4 py-2">
        <div class="text-xs font-medium text-[var(--text-secondary)] mb-2">Output Log</div>
        <pre
          v-if="status?.output"
          ref="logEl"
          class="text-xs font-mono text-[var(--text-secondary)] whitespace-pre-wrap max-h-64 overflow-y-auto bg-[var(--bg-primary)] rounded p-3 border border-[var(--border)]"
        >{{ status.output }}</pre>
        <div v-else-if="error" class="text-xs text-[var(--danger)]">{{ error }}</div>
        <div v-else class="text-xs text-[var(--text-secondary)]">No output available yet</div>
      </div>
    </div>
  </div>
</template>
