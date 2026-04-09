<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { usePolling } from '../../composables/usePolling.js'
import * as api from '../../api/client.js'
import { ScrollText, Filter, RefreshCw, ChevronDown } from 'lucide-vue-next'

const expandedEvent = ref(null)

function toggleExpand(id) {
  expandedEvent.value = expandedEvent.value === id ? null : id
}

const store = useVmStore()
const events = ref([])
const loading = ref(true)
const loadingMore = ref(false)
const hasMore = ref(false)
const nextBefore = ref('')
const total = ref(0)

// Filters
const category = ref('')
const actor = ref('')
const resource = ref('')
const timeRange = ref('')

const categories = [
  { value: '', label: 'All Categories' },
  { value: 'vm', label: 'VM' },
  { value: 'schedule', label: 'Schedule' },
  { value: 'ansible', label: 'Ansible' },
  { value: 'llm', label: 'LLM' },
  { value: 'config', label: 'Config' },
]

const actors = [
  { value: '', label: 'All Actors' },
  { value: 'user', label: 'User' },
  { value: 'scheduler', label: 'Scheduler' },
  { value: 'llm_agent', label: 'LLM Agent' },
]

const timeRanges = [
  { value: '', label: 'All Time' },
  { value: '1h', label: 'Last Hour' },
  { value: '24h', label: 'Last 24h' },
  { value: '7d', label: 'Last 7 Days' },
]

function sinceFromRange(range) {
  if (!range) return ''
  const now = new Date()
  switch (range) {
    case '1h': return new Date(now - 3600000).toISOString()
    case '24h': return new Date(now - 86400000).toISOString()
    case '7d': return new Date(now - 604800000).toISOString()
    default: return ''
  }
}

function buildParams() {
  return {
    category: category.value,
    actor: actor.value,
    resource: resource.value,
    since: sinceFromRange(timeRange.value),
    limit: 50,
  }
}

async function fetchEvents() {
  try {
    const result = await api.getEvents(buildParams())
    events.value = result.events || []
    hasMore.value = result.has_more
    nextBefore.value = result.next_before || ''
    total.value = result.total || 0
  } catch { /* ignore */ }
  loading.value = false
}

async function loadMore() {
  if (!hasMore.value || loadingMore.value) return
  loadingMore.value = true
  try {
    const params = buildParams()
    params.before = nextBefore.value
    const result = await api.getEvents(params)
    events.value = [...events.value, ...(result.events || [])]
    hasMore.value = result.has_more
    nextBefore.value = result.next_before || ''
  } catch { /* ignore */ }
  loadingMore.value = false
}

// Debounced resource filter
let resourceTimer = null
const resourceInput = ref('')
watch(resourceInput, (val) => {
  clearTimeout(resourceTimer)
  resourceTimer = setTimeout(() => {
    resource.value = val
  }, 300)
})

// Re-fetch when filters change
watch([category, actor, resource, timeRange], () => {
  loading.value = true
  fetchEvents()
})

// Poll only when this panel is visible
usePolling(() => {
  if (store.selectedNode === '__events__') {
    fetchEvents()
  }
}, 10000)

onMounted(fetchEvents)

function formatTime(ts) {
  try {
    const d = new Date(ts)
    return d.toLocaleString(undefined, {
      month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    })
  } catch {
    return ts
  }
}

const categoryColors = {
  vm: 'bg-blue-500/20 text-blue-400',
  schedule: 'bg-purple-500/20 text-purple-400',
  ansible: 'bg-orange-500/20 text-orange-400',
  llm: 'bg-green-500/20 text-green-400',
  config: 'bg-gray-500/20 text-gray-400',
}

const resultColors = {
  success: 'text-[var(--success)]',
  failed: 'text-[var(--danger)]',
  partial: 'text-[var(--warning)]',
  no_targets: 'text-[var(--text-secondary)]',
}

function categoryClass(cat) {
  return categoryColors[cat] || 'bg-gray-500/20 text-gray-400'
}

function resultClass(result) {
  return resultColors[result] || 'text-[var(--text-secondary)]'
}
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
      <div class="flex items-center gap-2">
        <ScrollText class="w-5 h-5 text-[var(--accent)]" />
        <h2 class="text-lg font-semibold">Event Log</h2>
        <span v-if="total > 0" class="text-xs text-[var(--text-secondary)]">({{ total }} total)</span>
      </div>
      <button
        class="p-1.5 rounded hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]"
        title="Refresh"
        @click="fetchEvents"
      >
        <RefreshCw class="w-4 h-4" />
      </button>
    </div>

    <!-- Filters -->
    <div class="flex flex-wrap items-center gap-2 px-6 py-3 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
      <Filter class="w-4 h-4 text-[var(--text-secondary)] flex-shrink-0" />
      <select
        v-model="category"
        class="text-xs bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2 py-1.5 text-[var(--text-primary)]"
      >
        <option v-for="c in categories" :key="c.value" :value="c.value">{{ c.label }}</option>
      </select>
      <select
        v-model="actor"
        class="text-xs bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2 py-1.5 text-[var(--text-primary)]"
      >
        <option v-for="a in actors" :key="a.value" :value="a.value">{{ a.label }}</option>
      </select>
      <input
        v-model="resourceInput"
        type="text"
        placeholder="Filter resource..."
        class="text-xs bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2 py-1.5 text-[var(--text-primary)] w-36"
      />
      <div class="flex gap-1">
        <button
          v-for="t in timeRanges"
          :key="t.value"
          class="text-xs px-2 py-1 rounded transition-colors"
          :class="timeRange === t.value
            ? 'bg-[var(--accent)]/20 text-[var(--accent)]'
            : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
          @click="timeRange = t.value"
        >
          {{ t.label }}
        </button>
      </div>
    </div>

    <!-- Content -->
    <div v-if="loading" class="flex-1 flex items-center justify-center text-[var(--text-secondary)] text-sm">
      Loading...
    </div>

    <div v-else-if="events.length === 0" class="flex-1 flex items-center justify-center text-[var(--text-secondary)] text-sm">
      No events found.
    </div>

    <div v-else class="flex-1 overflow-y-auto">
      <div class="divide-y divide-[var(--border)]">
        <div v-for="e in events" :key="e.id">
          <div class="flex items-center gap-3 px-6 py-2.5 hover:bg-[var(--bg-hover)] text-xs">
            <!-- Timestamp -->
            <span class="text-[var(--text-secondary)] flex-shrink-0 w-36 font-mono">
              {{ formatTime(e.timestamp) }}
            </span>

            <!-- Category badge -->
            <span
              class="flex-shrink-0 px-1.5 py-0.5 rounded text-[10px] font-medium uppercase"
              :class="categoryClass(e.category)"
            >
              {{ e.category }}
            </span>

            <!-- Actor -->
            <span class="flex-shrink-0 w-16 text-[var(--text-secondary)]">
              {{ e.actor }}
            </span>

            <!-- Action -->
            <span class="font-medium text-[var(--text-primary)] flex-shrink-0 w-28">
              {{ e.action }}
            </span>

            <!-- Resource -->
            <span class="text-[var(--text-primary)] truncate min-w-0 flex-1">
              {{ e.resource }}
            </span>

            <!-- Result -->
            <span class="flex-shrink-0 font-medium" :class="resultClass(e.result)">
              {{ e.result }}
            </span>

            <!-- API Call button -->
            <button
              v-if="e.endpoint || e.detail"
              class="flex items-center gap-1 flex-shrink-0 px-2 py-0.5 rounded text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] transition-colors"
              @click="toggleExpand(e.id)"
            >
              <span>API Call</span>
              <ChevronDown
                class="w-3 h-3 transition-transform"
                :class="expandedEvent === e.id ? 'rotate-180' : ''"
              />
            </button>
          </div>

          <!-- Expanded detail row -->
          <div
            v-if="expandedEvent === e.id"
            class="px-6 pb-2.5 pt-0 ml-36 text-xs"
          >
            <div class="rounded bg-[var(--bg-primary)] border border-[var(--border)] px-3 py-2 space-y-1">
              <div v-if="e.endpoint" class="flex gap-2">
                <span class="text-[var(--text-secondary)] flex-shrink-0">Endpoint:</span>
                <span class="font-mono text-[var(--text-primary)]">{{ e.endpoint }}</span>
              </div>
              <div v-if="e.detail" class="flex gap-2">
                <span class="text-[var(--text-secondary)] flex-shrink-0">Detail:</span>
                <span class="text-[var(--text-primary)]">{{ e.detail }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Load more -->
      <div v-if="hasMore" class="px-6 py-3 text-center border-t border-[var(--border)]">
        <button
          class="text-xs text-[var(--accent)] hover:underline"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{ loadingMore ? 'Loading...' : 'Load more events' }}
        </button>
      </div>
    </div>
  </div>
</template>
