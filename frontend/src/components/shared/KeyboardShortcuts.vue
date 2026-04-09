<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { Keyboard } from 'lucide-vue-next'

const open = ref(false)
const panelRef = ref(null)

const isMac = navigator.platform.includes('Mac')
const mod = isMac ? 'Cmd' : 'Ctrl'

const shortcuts = [
  { keys: `${mod}+F`, action: 'Find' },
  { keys: `${mod}+H`, action: 'Find & Replace' },
  { keys: `${mod}+G`, action: 'Next match' },
  { keys: `${mod}+Z`, action: 'Undo' },
  { keys: `${mod}+Shift+Z`, action: 'Redo' },
  { keys: 'Tab', action: 'Indent' },
  { keys: 'Shift+Tab', action: 'Dedent' },
  { keys: 'Esc', action: 'Close search / exit fullscreen' },
]

function onClickOutside(e) {
  if (panelRef.value && !panelRef.value.contains(e.target)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onClickOutside))
onUnmounted(() => document.removeEventListener('mousedown', onClickOutside))
</script>

<template>
  <div ref="panelRef" class="relative">
    <button
      @click="open = !open"
      class="p-1.5 rounded hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
      title="Keyboard shortcuts"
    >
      <Keyboard class="w-4 h-4" />
    </button>
    <div
      v-if="open"
      class="absolute right-0 top-full mt-1 w-56 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg shadow-lg z-50 py-2 px-3"
    >
      <div class="text-[10px] uppercase tracking-wider text-[var(--muted)] mb-1.5">Shortcuts</div>
      <div
        v-for="s in shortcuts"
        :key="s.action"
        class="flex items-center justify-between py-1 text-xs"
      >
        <span class="text-[var(--text-secondary)]">{{ s.action }}</span>
        <kbd class="px-1.5 py-0.5 rounded bg-[var(--bg-primary)] border border-[var(--border)] text-[10px] font-mono text-[var(--text-primary)]">{{ s.keys }}</kbd>
      </div>
    </div>
  </div>
</template>
