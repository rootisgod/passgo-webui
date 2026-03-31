<script setup>
import { useToastStore } from '../../stores/toastStore.js'
import { CheckCircle, XCircle, Info, X } from 'lucide-vue-next'

const toasts = useToastStore()

const icons = {
  success: CheckCircle,
  error: XCircle,
  info: Info,
}

const colors = {
  success: 'border-[var(--success)] bg-green-900/40',
  error: 'border-[var(--danger)] bg-red-900/40',
  info: 'border-[var(--accent)] bg-blue-900/40',
}
</script>

<template>
  <div class="fixed top-4 right-4 z-50 flex flex-col gap-2 min-w-[280px]">
    <TransitionGroup name="toast">
      <div
        v-for="toast in toasts.toasts"
        :key="toast.id"
        class="flex items-center gap-3 px-4 py-3 rounded-lg border shadow-lg backdrop-blur"
        :class="colors[toast.type]"
      >
        <component :is="icons[toast.type]" class="w-5 h-5 flex-shrink-0" />
        <span class="text-sm flex-1">{{ toast.message }}</span>
        <button @click="toasts.remove(toast.id)" class="opacity-50 hover:opacity-100">
          <X class="w-4 h-4" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
