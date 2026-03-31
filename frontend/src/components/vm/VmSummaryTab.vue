<script setup>
import { computed, ref } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import StatusDot from '../shared/StatusDot.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import CloudInitStatus from './CloudInitStatus.vue'
import { Play, Square, Pause, Trash2, RotateCcw } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const confirmAction = ref(null)

const vm = computed(() => store.selectedVm)

const stateColors = {
  Running: 'bg-green-900/30 text-[var(--success)] border-green-800',
  Stopped: 'bg-gray-800/30 text-[var(--muted)] border-gray-700',
  Suspended: 'bg-yellow-900/30 text-[var(--warning)] border-yellow-800',
  Deleted: 'bg-red-900/30 text-[var(--danger)] border-red-800',
  Creating: 'bg-blue-900/30 text-[var(--accent)] border-blue-800',
}

const properties = computed(() => {
  if (!vm.value) return []
  const v = vm.value
  return [
    { label: 'Name', value: v.name },
    { label: 'State', value: v.state },
    { label: 'IP Address', value: v.ipv4?.join(', ') || '—' },
    { label: 'Release', value: v.release || '—' },
    { label: 'Image', value: v.image_hash ? v.image_hash.substring(0, 12) : '—' },
    { label: 'CPUs', value: v.cpus || '—' },
    { label: 'Memory', value: v.memory_usage && v.memory_total ? `${v.memory_usage} / ${v.memory_total}` : '—' },
    { label: 'Disk', value: v.disk_usage && v.disk_total ? `${v.disk_usage} / ${v.disk_total}` : '—' },
    { label: 'Load', value: v.load || '—' },
    { label: 'Snapshots', value: String(v.snapshots ?? 0) },
    { label: 'Mounts', value: String(v.mounts?.length ?? 0) },
  ]
})

async function action(fn, msg) {
  try {
    await fn()
    toasts.success(msg)
    store.fetchVMs()
  } catch (e) { toasts.error(e.message) }
}

function confirmDanger(fn, message) {
  confirmAction.value = { fn, message }
}

async function executeConfirmed() {
  const fn = confirmAction.value?.fn
  confirmAction.value = null
  if (fn) await fn()
}

const isRunning = computed(() => vm.value?.state === 'Running')
const isStopped = computed(() => vm.value?.state === 'Stopped')
const isSuspended = computed(() => vm.value?.state === 'Suspended')
const isDeleted = computed(() => vm.value?.state === 'Deleted')
</script>

<template>
  <div class="p-6" v-if="vm">
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Properties -->
      <div class="lg:col-span-2">
        <div class="flex items-center gap-3 mb-4">
          <h3 class="text-lg font-semibold">{{ vm.name }}</h3>
          <span
            class="px-2 py-0.5 rounded text-xs border flex items-center gap-1.5"
            :class="stateColors[vm.state] || stateColors.Stopped"
          >
            <StatusDot :state="vm.state" />
            {{ vm.state }}
          </span>
        </div>

        <div class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] divide-y divide-[var(--border)]">
          <div
            v-for="prop in properties"
            :key="prop.label"
            class="flex px-4 py-2.5 text-sm"
          >
            <span class="w-32 text-[var(--text-secondary)] flex-shrink-0">{{ prop.label }}</span>
            <span class="text-[var(--text-primary)]">{{ prop.value }}</span>
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div>
        <h3 class="text-lg font-semibold mb-4">Actions</h3>
        <div class="flex flex-col gap-2">
          <ActionButton
            label="Start"
            :icon="Play"
            variant="success"
            :disabled="isRunning"
            @click="action(() => api.startVM(vm.name), `${vm.name} started`)"
          />
          <ActionButton
            label="Stop"
            :icon="Square"
            :disabled="isStopped || isDeleted"
            @click="action(() => api.stopVM(vm.name), `${vm.name} stopped`)"
          />
          <ActionButton
            label="Suspend"
            :icon="Pause"
            :disabled="!isRunning"
            @click="action(() => api.suspendVM(vm.name), `${vm.name} suspended`)"
          />
          <ActionButton
            label="Delete"
            :icon="Trash2"
            variant="danger"
            :disabled="isDeleted"
            @click="confirmDanger(
              () => action(() => api.deleteVM(vm.name), `${vm.name} deleted`),
              `Delete VM '${vm.name}'?`
            )"
          />
          <ActionButton
            v-if="isDeleted"
            label="Recover"
            :icon="RotateCcw"
            variant="success"
            @click="action(() => api.recoverVM(vm.name), `${vm.name} recovered`)"
          />
        </div>
      </div>
    </div>

    <!-- Cloud-Init Status -->
    <CloudInitStatus v-if="isRunning" :vm-name="vm.name" :key="'ci-' + vm.name" class="mt-6" />

    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />
  </div>
</template>
