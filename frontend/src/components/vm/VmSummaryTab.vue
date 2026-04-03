<script setup>
import { computed, ref } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { getHistory } from '../../composables/useMetricsHistory.js'
import ActionButton from '../shared/ActionButton.vue'
import StatusDot from '../shared/StatusDot.vue'
import Sparkline from '../shared/Sparkline.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import CloudInitStatus from './CloudInitStatus.vue'
import CloneVmModal from '../modals/CloneVmModal.vue'
import { Play, Square, Pause, Copy, Trash2, RotateCcw, Cpu, MemoryStick, HardDrive } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const confirmAction = ref(null)
const showCloneModal = ref(false)

const vm = computed(() => store.selectedVm)

const stateColors = {
  Running: 'bg-green-900/30 text-[var(--success)] border-green-800',
  Stopped: 'bg-gray-800/30 text-[var(--muted)] border-gray-700',
  Suspended: 'bg-yellow-900/30 text-[var(--warning)] border-yellow-800',
  Deleted: 'bg-red-900/30 text-[var(--danger)] border-red-800',
  Creating: 'bg-blue-900/30 text-[var(--accent)] border-blue-800',
}

// Resource metrics
const memoryPercent = computed(() => {
  if (!vm.value?.memory_total_raw) return 0
  return Math.round((vm.value.memory_usage_raw / vm.value.memory_total_raw) * 100)
})

const diskPercent = computed(() => {
  if (!vm.value?.disk_total_raw) return 0
  return Math.round((vm.value.disk_usage_raw / vm.value.disk_total_raw) * 100)
})

const loadValues = computed(() => {
  if (!vm.value?.load) return null
  const parts = vm.value.load.split(' ').map(Number)
  return parts.length === 3 ? { one: parts[0], five: parts[1], fifteen: parts[2] } : null
})

function barColor(percent) {
  if (percent > 90) return 'bg-[var(--danger)]'
  if (percent > 70) return 'bg-[var(--warning)]'
  return 'bg-[var(--success)]'
}

function loadColor(load, cpus) {
  const ratio = load / (parseInt(cpus) || 1)
  if (ratio > 1) return 'text-[var(--danger)]'
  if (ratio > 0.7) return 'text-[var(--warning)]'
  return 'text-[var(--success)]'
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

const metrics = computed(() => vm.value ? getHistory(vm.value.name) : { cpu: [], memory: [], disk: [] })

const isRunning = computed(() => vm.value?.state === 'Running')
const isStopped = computed(() => vm.value?.state === 'Stopped')
const isSuspended = computed(() => vm.value?.state === 'Suspended')
const isDeleted = computed(() => vm.value?.state === 'Deleted')
</script>

<template>
  <div class="p-6" v-if="vm">
    <!-- Resource Cards (only when running) -->
    <div v-if="isRunning && loadValues" class="grid grid-cols-3 gap-4 mb-6">
      <!-- CPU Load -->
      <div class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-4">
        <div class="flex items-center gap-2 text-xs text-[var(--text-secondary)] mb-2">
          <Cpu class="w-4 h-4" />
          CPU Load
        </div>
        <div class="text-2xl font-bold mb-1" :class="loadColor(loadValues.one, vm.cpus)">
          {{ loadValues.one.toFixed(2) }}
        </div>
        <div class="text-xs text-[var(--text-secondary)] mb-2">
          {{ loadValues.five.toFixed(2) }} <span class="text-[var(--muted)]">5m</span>
          {{ loadValues.fifteen.toFixed(2) }} <span class="text-[var(--muted)]">15m</span>
          <span class="ml-2 text-[var(--muted)]">/ {{ vm.cpus }} CPUs</span>
        </div>
        <Sparkline :data="metrics.cpu" :max="parseInt(vm.cpus) || 1" color="var(--accent)" :height="28" />
      </div>

      <!-- Memory -->
      <div class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-4">
        <div class="flex items-center gap-2 text-xs text-[var(--text-secondary)] mb-2">
          <MemoryStick class="w-4 h-4" />
          Memory
        </div>
        <div class="text-2xl font-bold mb-1">{{ memoryPercent }}%</div>
        <div class="w-full h-1.5 rounded-full bg-[var(--bg-primary)] mb-2">
          <div class="h-full rounded-full transition-all" :class="barColor(memoryPercent)" :style="{ width: memoryPercent + '%' }" />
        </div>
        <div class="text-xs text-[var(--text-secondary)] mb-2">{{ vm.memory_usage }} / {{ vm.memory_total }}</div>
        <Sparkline :data="metrics.memory" :max="100" color="var(--success)" :height="28" />
      </div>

      <!-- Disk -->
      <div class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-4">
        <div class="flex items-center gap-2 text-xs text-[var(--text-secondary)] mb-2">
          <HardDrive class="w-4 h-4" />
          Disk
        </div>
        <div class="text-2xl font-bold mb-1">{{ diskPercent }}%</div>
        <div class="w-full h-1.5 rounded-full bg-[var(--bg-primary)] mb-2">
          <div class="h-full rounded-full transition-all" :class="barColor(diskPercent)" :style="{ width: diskPercent + '%' }" />
        </div>
        <div class="text-xs text-[var(--text-secondary)] mb-2">{{ vm.disk_usage }} / {{ vm.disk_total }}</div>
        <Sparkline :data="metrics.disk" :max="100" color="var(--warning)" :height="28" />
      </div>
    </div>

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
            :disabled="isRunning || isDeleted"
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
            label="Clone"
            :icon="Copy"
            :disabled="!isStopped"
            @click="showCloneModal = true"
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

    <CloneVmModal
      v-if="showCloneModal"
      :vm-name="vm.name"
      @close="showCloneModal = false"
      @cloned="showCloneModal = false"
    />
  </div>
</template>
