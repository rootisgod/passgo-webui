<script setup>
defineProps({
  message: { type: String, required: true },
})

const emit = defineEmits(['confirm', 'cancel'])
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-40 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="emit('cancel')" />
      <div class="relative bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-6 max-w-md w-full mx-4 shadow-2xl">
        <p class="text-sm mb-6">{{ message }}</p>
        <div class="flex justify-end gap-3">
          <button
            @click="emit('cancel')"
            class="px-4 py-2 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
          >Cancel</button>
          <button
            @click="emit('confirm')"
            class="px-4 py-2 text-sm rounded bg-[var(--danger)] hover:bg-red-600 transition-colors"
          >Confirm</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
