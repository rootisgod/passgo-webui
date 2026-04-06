<script setup>
import { ref, onMounted } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import { Save } from 'lucide-vue-next'

const toasts = useToastStore()
const loading = ref(true)
const saving = ref(false)

const cpus = ref(2)
const memoryMB = ref(1024)
const diskGB = ref(8)

onMounted(async () => {
  try {
    const defaults = await api.getVMDefaults()
    cpus.value = defaults.cpus
    memoryMB.value = defaults.memory_mb
    diskGB.value = defaults.disk_gb
  } catch (e) {
    toasts.error('Failed to load VM defaults')
  } finally {
    loading.value = false
  }
})

async function save() {
  saving.value = true
  try {
    const updated = await api.updateVMDefaults({
      cpus: Number(cpus.value),
      memory_mb: Number(memoryMB.value),
      disk_gb: Number(diskGB.value),
    })
    cpus.value = updated.cpus
    memoryMB.value = updated.memory_mb
    diskGB.value = updated.disk_gb
    toasts.success('VM defaults saved')
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="p-6">
    <h2 class="text-xl font-semibold mb-6">Settings</h2>

    <div v-if="loading" class="text-sm text-[var(--text-secondary)]">Loading...</div>

    <div v-else>
      <h3 class="text-sm font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-4">Default VM Specs</h3>
      <p class="text-xs text-[var(--muted)] mb-6">These values are used as defaults when creating new VMs.</p>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-6 max-w-2xl mb-6">
        <!-- CPUs -->
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">CPUs</label>
          <input
            v-model.number="cpus"
            type="number"
            min="1"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          />
        </div>

        <!-- Memory -->
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">Memory (MB)</label>
          <input
            v-model.number="memoryMB"
            type="number"
            min="512"
            step="256"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          />
        </div>

        <!-- Disk -->
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">Disk (GB)</label>
          <input
            v-model.number="diskGB"
            type="number"
            min="1"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          />
        </div>
      </div>

      <ActionButton label="Save" :icon="Save" variant="success" :disabled="saving" @click="save" />
    </div>
  </div>
</template>
