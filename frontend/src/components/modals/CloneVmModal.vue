<script setup>
import { ref, computed } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import { useVmStore } from '../../stores/vmStore.js'
import { cloneVM } from '../../api/client.js'

const props = defineProps({
  vmName: { type: String, required: true },
  snapshotName: { type: String, default: '' },
})
const emit = defineEmits(['close', 'cloned'])

const toasts = useToastStore()
const store = useVmStore()
const destName = ref('')
const submitting = ref(false)

const suggestedName = computed(() => {
  const existing = store.vms.map(v => v.name)
  for (let i = 1; ; i++) {
    const candidate = `${props.vmName}-clone${i}`
    if (!existing.includes(candidate)) return candidate
  }
})

async function submit() {
  submitting.value = true
  try {
    const name = destName.value.trim() || undefined
    await cloneVM(props.vmName, name, props.snapshotName || undefined)
    const label = destName.value.trim() || suggestedName.value
    toasts.success(`Cloning '${props.vmName}' as '${label}'`)
    store.fetchVMs()
    emit('cloned')
    emit('close')
  } catch (e) {
    toasts.error(e.message)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-40 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="emit('close')" />
      <div class="relative bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-6 max-w-md w-full mx-4 shadow-2xl">
        <h3 class="text-lg font-semibold mb-4">Clone Virtual Machine</h3>

        <div class="space-y-4">
          <div class="text-sm text-[var(--text-secondary)]">
            Source: <span class="font-mono text-[var(--text-primary)]">{{ vmName }}</span>
          </div>

          <div v-if="snapshotName" class="text-sm text-[var(--text-secondary)]">
            Restore to snapshot: <span class="font-mono text-[var(--text-primary)]">{{ snapshotName }}</span>
          </div>

          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Clone Name</label>
            <input
              v-model="destName"
              type="text"
              :placeholder="suggestedName"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm font-mono text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
              autofocus
              @keyup.enter="submit"
            />
            <p class="text-xs text-[var(--muted)] mt-1">Leave empty for auto-generated name</p>
          </div>
        </div>

        <div class="flex justify-end gap-3 mt-6">
          <button
            @click="emit('close')"
            class="px-4 py-2 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
          >Cancel</button>
          <button
            @click="submit"
            :disabled="submitting"
            class="px-4 py-2 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
          >Clone</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
