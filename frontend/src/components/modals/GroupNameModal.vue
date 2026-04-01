<script setup>
import { ref, onMounted } from 'vue'

const props = defineProps({
  mode: { type: String, default: 'create' }, // 'create' or 'rename'
  initialName: { type: String, default: '' },
})

const emit = defineEmits(['confirm', 'cancel'])
const name = ref(props.initialName)
const inputRef = ref(null)

function submit() {
  const trimmed = name.value.trim()
  if (trimmed) emit('confirm', trimmed)
}

onMounted(() => {
  inputRef.value?.focus()
  if (props.mode === 'rename') inputRef.value?.select()
})
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-40 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="emit('cancel')" />
      <div class="relative bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-6 max-w-sm w-full mx-4 shadow-2xl">
        <h3 class="text-sm font-medium mb-4">{{ mode === 'create' ? 'New Group' : 'Rename Group' }}</h3>
        <input
          ref="inputRef"
          v-model="name"
          @keydown.enter="submit"
          @keydown.escape="emit('cancel')"
          placeholder="Group name"
          class="w-full px-3 py-2 text-sm rounded bg-[var(--bg-primary)] border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none"
        />
        <div class="flex justify-end gap-3 mt-4">
          <button
            @click="emit('cancel')"
            class="px-4 py-2 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
          >Cancel</button>
          <button
            @click="submit"
            :disabled="!name.trim()"
            class="px-4 py-2 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
          >{{ mode === 'create' ? 'Create' : 'Rename' }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
