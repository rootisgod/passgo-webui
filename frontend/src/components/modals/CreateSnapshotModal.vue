<script setup>
import { ref } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import { useVmStore } from '../../stores/vmStore.js'
import { createSnapshot } from '../../api/client.js'

const props = defineProps({
  vmName: { type: String, required: true },
})
const emit = defineEmits(['close', 'created'])

const toasts = useToastStore()
const store = useVmStore()
const name = ref('')
const comment = ref('')
const submitting = ref(false)

async function submit() {
  if (!name.value.trim()) return
  submitting.value = true
  const trimmed = name.value.trim()
  try {
    await createSnapshot(props.vmName, trimmed, comment.value.trim())
    toasts.success(`Snapshot '${trimmed}' created`)
    store.fetchVMs()
    // Emit the snapshot name so the parent can mark it as current —
    // a freshly taken snapshot IS the current one until a restore moves away.
    emit('created', trimmed)
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
        <h3 class="text-lg font-semibold mb-4">Create Snapshot</h3>

        <div class="space-y-4">
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Name *</label>
            <input
              v-model="name"
              type="text"
              placeholder="snapshot-1"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
              autofocus
              @keyup.enter="submit"
            />
          </div>
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Comment</label>
            <input
              v-model="comment"
              type="text"
              placeholder="Optional description"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
          </div>
        </div>

        <div class="flex justify-end gap-3 mt-6">
          <button
            @click="emit('close')"
            class="px-4 py-2 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
          >Cancel</button>
          <button
            @click="submit"
            :disabled="!name.trim() || submitting"
            class="px-4 py-2 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
          >Create</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
