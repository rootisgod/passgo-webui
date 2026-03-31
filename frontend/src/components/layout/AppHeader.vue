<script setup>
import { useVmStore } from '../../stores/vmStore.js'
import { Server } from 'lucide-vue-next'

const store = useVmStore()
</script>

<template>
  <header class="flex items-center justify-between px-4 py-2 bg-[var(--bg-secondary)] border-b border-[var(--border)]">
    <div class="flex items-center gap-3">
      <Server class="w-5 h-5 text-[var(--accent)]" />
      <span class="font-semibold text-lg">PassGo Web</span>
    </div>
    <div class="flex items-center gap-4 text-sm text-[var(--text-secondary)]">
      <span>{{ store.hostname }}</span>
    </div>
  </header>
</template>
