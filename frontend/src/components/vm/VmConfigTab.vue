<script setup>
import { ref, computed, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import { getVM, getVMConfig, resizeVM, getHostResources } from '../../api/client.js'
import { Loader2, AlertTriangle } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()

const loading = ref(true)
const saving = ref(false)
const vmData = ref(null)
const hostRes = ref(null)
const errors = ref({})

// Form values
const cpus = ref(1)
const memoryMB = ref(1024)
const diskGB = ref(8)

// Original values for change detection
const origCpus = ref(1)
const origMemoryMB = ref(1024)
const origDiskGB = ref(8)

const isStopped = computed(() => vmData.value?.state === 'Stopped')
const isDeleted = computed(() => vmData.value?.state === 'Deleted')
const currentDiskGB = computed(() => origDiskGB.value)

const hasChanges = computed(() =>
  cpus.value !== origCpus.value ||
  memoryMB.value !== origMemoryMB.value ||
  diskGB.value !== origDiskGB.value
)

onMounted(async () => {
  try {
    const [vm, cfg, host] = await Promise.all([
      getVM(store.selectedNode),
      getVMConfig(store.selectedNode),
      getHostResources().catch(() => null),
    ])
    vmData.value = vm
    hostRes.value = host

    // Use multipass get values (cfg) — these are correct even when VM is stopped
    origCpus.value = cpus.value = cfg.cpus || parseInt(vm.cpus) || 1
    origMemoryMB.value = memoryMB.value = cfg.memory_mb || Math.round(vm.memory_total_raw / (1024 * 1024))
    origDiskGB.value = diskGB.value = cfg.disk_gb || Math.floor(vm.disk_total_raw / (1024 * 1024 * 1024))
  } catch (e) {
    toasts.error('Failed to load VM info: ' + e.message)
  } finally {
    loading.value = false
  }
})

function validate() {
  errors.value = {}
  if (cpus.value < 1) errors.value.cpus = 'Must be at least 1'
  if (memoryMB.value < 256) errors.value.memory = 'Must be at least 256 MB'
  if (hostRes.value && memoryMB.value > hostRes.value.total_memory_mb) {
    errors.value.memory = `Exceeds host capacity (${hostRes.value.total_memory_mb} MB)`
  }
  if (diskGB.value < currentDiskGB.value) {
    errors.value.disk = `Cannot decrease (current: ${currentDiskGB.value} GB)`
  }
  if (diskGB.value < 1) errors.value.disk = 'Must be at least 1 GB'
  return Object.keys(errors.value).length === 0
}

async function save() {
  if (!validate()) return
  saving.value = true
  try {
    const payload = {}
    if (cpus.value !== origCpus.value) payload.cpus = cpus.value
    if (memoryMB.value !== origMemoryMB.value) payload.memory_mb = memoryMB.value
    if (diskGB.value !== origDiskGB.value) payload.disk_gb = diskGB.value

    await resizeVM(store.selectedNode, payload)
    toasts.success('VM configuration updated')

    // Refresh to pick up new values
    origCpus.value = cpus.value
    origMemoryMB.value = memoryMB.value
    origDiskGB.value = diskGB.value
    errors.value = {}
    store.fetchVMs()
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="p-6">
    <h3 class="text-lg font-semibold mb-1">Configuration</h3>
    <p class="text-xs text-[var(--text-secondary)] mb-4">Resize CPU, memory, and disk for this instance.</p>

    <div v-if="loading" class="text-[var(--text-secondary)] text-sm">Loading...</div>

    <template v-else-if="vmData">
      <!-- Deleted warning -->
      <div v-if="isDeleted" class="flex items-start gap-2 bg-red-500/10 border border-red-500/30 rounded-lg px-4 py-3 mb-5">
        <AlertTriangle class="w-4 h-4 text-red-400 mt-0.5 shrink-0" />
        <p class="text-xs text-red-300">This VM has been deleted. Recover it to make configuration changes.</p>
      </div>

      <!-- State warning -->
      <div v-else-if="!isStopped" class="flex items-start gap-2 bg-amber-500/10 border border-amber-500/30 rounded-lg px-4 py-3 mb-5">
        <AlertTriangle class="w-4 h-4 text-amber-400 mt-0.5 shrink-0" />
        <p class="text-xs text-amber-300">CPU and memory changes require the VM to be stopped. Disk can be resized while running.</p>
      </div>

      <div class="grid grid-cols-3 gap-4 mb-6">
        <!-- CPUs -->
        <div>
          <label class="block text-xs text-[var(--text-secondary)] mb-1">CPUs</label>
          <input
            v-model.number="cpus"
            type="number"
            :min="1"
            :disabled="!isStopped || isDeleted"
            class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)] disabled:opacity-50 disabled:cursor-not-allowed"
          />
          <p v-if="hostRes" class="text-xs text-[var(--text-secondary)] mt-1">Host has {{ hostRes.total_cpus }} cores</p>
          <p v-if="errors.cpus" class="text-xs text-red-400 mt-1">{{ errors.cpus }}</p>
        </div>

        <!-- Memory -->
        <div>
          <label class="block text-xs text-[var(--text-secondary)] mb-1">Memory (MB)</label>
          <input
            v-model.number="memoryMB"
            type="number"
            :min="256"
            :step="256"
            :disabled="!isStopped || isDeleted"
            class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)] disabled:opacity-50 disabled:cursor-not-allowed"
          />
          <p v-if="hostRes" class="text-xs text-[var(--text-secondary)] mt-1">Host has {{ hostRes.total_memory_mb }} MB total</p>
          <p v-if="errors.memory" class="text-xs text-red-400 mt-1">{{ errors.memory }}</p>
        </div>

        <!-- Disk -->
        <div>
          <label class="block text-xs text-[var(--text-secondary)] mb-1">Disk (GB)</label>
          <input
            v-model.number="diskGB"
            type="number"
            :min="currentDiskGB"
            :disabled="isDeleted"
            class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)] disabled:opacity-50 disabled:cursor-not-allowed"
          />
          <p class="text-xs text-[var(--text-secondary)] mt-1">Can only increase. Current: {{ currentDiskGB }} GB</p>
          <p v-if="errors.disk" class="text-xs text-red-400 mt-1">{{ errors.disk }}</p>
        </div>
      </div>

      <button
        @click="save"
        :disabled="!hasChanges || saving || isDeleted"
        class="px-4 py-2 bg-[var(--accent)] text-white text-sm rounded hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-2"
      >
        <Loader2 v-if="saving" class="w-4 h-4 animate-spin" />
        Apply Changes
      </button>
    </template>
  </div>
</template>
