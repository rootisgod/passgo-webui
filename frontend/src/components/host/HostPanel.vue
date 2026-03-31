<script setup>
import { ref } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import CreateVmModal from '../modals/CreateVmModal.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import { Plus, Play, Square, Trash2, Server, Activity, Pause, AlertTriangle, X, Loader2 } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const showCreateModal = ref(false)
const confirmAction = ref(null)

const cards = [
  { label: 'Total', getter: () => store.totalCount, icon: Server, color: 'text-[var(--accent)]' },
  { label: 'Running', getter: () => store.runningCount, icon: Activity, color: 'text-[var(--success)]' },
  { label: 'Stopped', getter: () => store.stoppedCount, icon: Square, color: 'text-[var(--muted)]' },
  { label: 'Suspended', getter: () => store.suspendedCount, icon: Pause, color: 'text-[var(--warning)]' },
  { label: 'Deleted', getter: () => store.deletedCount, icon: AlertTriangle, color: 'text-[var(--danger)]' },
]

async function doStartAll() {
  try {
    await api.startAll()
    toasts.success('All stopped VMs started')
    store.fetchVMs()
  } catch (e) { toasts.error(e.message) }
}

async function doStopAll() {
  try {
    await api.stopAll()
    toasts.success('All running VMs stopped')
    store.fetchVMs()
  } catch (e) { toasts.error(e.message) }
}

async function doPurge() {
  try {
    await api.purgeDeleted()
    toasts.success('Deleted VMs purged')
    store.fetchVMs()
  } catch (e) { toasts.error(e.message) }
}

function confirmBulk(action, message) {
  confirmAction.value = { action, message }
}

async function executeConfirmed() {
  const action = confirmAction.value?.action
  confirmAction.value = null
  if (action) await action()
}
</script>

<template>
  <div class="p-6">
    <h2 class="text-xl font-semibold mb-6">Dashboard</h2>

    <!-- Summary cards -->
    <div class="grid grid-cols-5 gap-4 mb-8">
      <div
        v-for="card in cards"
        :key="card.label"
        class="bg-[var(--bg-surface)] rounded-lg p-4 border border-[var(--border)]"
      >
        <div class="flex items-center gap-3">
          <component :is="card.icon" class="w-8 h-8" :class="card.color" />
          <div>
            <div class="text-2xl font-bold">{{ card.getter() }}</div>
            <div class="text-xs text-[var(--text-secondary)]">{{ card.label }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Launching VMs -->
    <div v-if="store.launchingCount > 0" class="mb-4 flex items-center gap-3 px-4 py-3 rounded-lg bg-blue-900/20 border border-blue-800/30">
      <Loader2 class="w-4 h-4 animate-spin text-[var(--accent)]" />
      <span class="text-sm text-[var(--accent)]">
        {{ store.launchingCount }} VM{{ store.launchingCount !== 1 ? 's' : '' }} launching...
      </span>
    </div>

    <!-- Failed launches -->
    <div
      v-for="launch in store.failedLaunches"
      :key="'fail-' + launch.name"
      class="mb-4 flex items-center gap-3 px-4 py-3 rounded-lg bg-red-900/20 border border-red-800/30"
    >
      <AlertTriangle class="w-4 h-4 text-[var(--danger)] flex-shrink-0" />
      <div class="flex-1 min-w-0">
        <span class="text-sm text-[var(--danger)] font-medium">{{ launch.name }}</span>
        <span class="text-sm text-[var(--text-secondary)]"> failed to launch: </span>
        <span class="text-sm text-[var(--text-secondary)]">{{ launch.error }}</span>
      </div>
      <button
        @click="store.dismissFailedLaunch(launch.name)"
        class="p-1 rounded hover:bg-red-900/30 transition-colors text-[var(--danger)]"
        title="Dismiss"
      >
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Actions -->
    <div class="flex items-center gap-3">
      <ActionButton label="Create VM" :icon="Plus" variant="success" @click="showCreateModal = true" />
      <ActionButton label="Start All" :icon="Play" @click="confirmBulk(doStartAll, 'Start all stopped VMs?')" />
      <ActionButton label="Stop All" :icon="Square" @click="confirmBulk(doStopAll, 'Stop all running VMs?')" />
      <ActionButton label="Purge Deleted" :icon="Trash2" variant="danger" @click="confirmBulk(doPurge, 'Permanently remove all deleted VMs?')" />
    </div>

    <CreateVmModal v-if="showCreateModal" @close="showCreateModal = false" />
    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />
  </div>
</template>
