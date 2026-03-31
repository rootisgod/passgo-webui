<script setup>
import { ref } from 'vue'
import { Loader2 } from 'lucide-vue-next'

const props = defineProps({
  label: String,
  icon: Object,
  variant: { type: String, default: 'default' }, // default, danger, success
  disabled: Boolean,
})

const emit = defineEmits(['click'])
const loading = ref(false)

async function handleClick() {
  if (loading.value || props.disabled) return
  loading.value = true
  try {
    await emit('click')
  } finally {
    // Small delay so the user sees the spinner
    setTimeout(() => { loading.value = false }, 300)
  }
}

const variants = {
  default: 'bg-[var(--bg-hover)] hover:bg-[var(--accent)] text-[var(--text-primary)]',
  danger: 'bg-red-900/30 hover:bg-[var(--danger)] text-red-300 hover:text-white',
  success: 'bg-green-900/30 hover:bg-[var(--success)] text-green-300 hover:text-white',
}
</script>

<template>
  <button
    class="flex items-center gap-2 px-3 py-1.5 rounded text-sm transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
    :class="variants[variant]"
    :disabled="disabled || loading"
    @click="handleClick"
  >
    <Loader2 v-if="loading" class="w-4 h-4 animate-spin" />
    <component v-else-if="icon" :is="icon" class="w-4 h-4" />
    <span>{{ label }}</span>
  </button>
</template>
