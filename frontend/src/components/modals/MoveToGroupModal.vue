<script setup>
import { useVmStore } from '../../stores/vmStore.js'
import { Folder, X as XIcon } from 'lucide-vue-next'

defineProps({
  vmName: { type: String, required: true },
  currentGroup: { type: String, default: '' },
})

const emit = defineEmits(['confirm', 'cancel'])
const store = useVmStore()
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-40 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="emit('cancel')" />
      <div class="relative bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-6 max-w-sm w-full mx-4 shadow-2xl">
        <h3 class="text-sm font-medium mb-1">Move "{{ vmName }}" to Group</h3>
        <p class="text-xs text-[var(--text-secondary)] mb-4">Select a group or remove from current group.</p>
        <div class="flex flex-col gap-1 max-h-60 overflow-y-auto">
          <!-- Ungrouped option -->
          <button
            @click="emit('confirm', '')"
            class="flex items-center gap-2 px-3 py-2 text-sm rounded hover:bg-[var(--bg-hover)] transition-colors text-left"
            :class="{ 'bg-[var(--bg-hover)]': !currentGroup }"
          >
            <XIcon class="w-4 h-4 text-[var(--text-secondary)]" />
            <span>Ungrouped</span>
          </button>
          <!-- Groups -->
          <button
            v-for="group in store.groups"
            :key="group"
            @click="emit('confirm', group)"
            class="flex items-center gap-2 px-3 py-2 text-sm rounded hover:bg-[var(--bg-hover)] transition-colors text-left"
            :class="{ 'bg-[var(--bg-hover)]': currentGroup === group }"
          >
            <Folder class="w-4 h-4 text-[var(--text-secondary)]" />
            <span>{{ group }}</span>
          </button>
        </div>
        <div class="flex justify-end mt-4">
          <button
            @click="emit('cancel')"
            class="px-4 py-2 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
          >Cancel</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
