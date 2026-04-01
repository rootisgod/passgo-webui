<script setup>
import { ref, onMounted, onUnmounted, nextTick, markRaw } from 'vue'

const props = defineProps({
  x: { type: Number, required: true },
  y: { type: Number, required: true },
  items: { type: Array, required: true },
  // items: [{ label, icon, action, variant?, disabled? }]
})
const emit = defineEmits(['close'])

const menuRef = ref(null)
const posX = ref(props.x)
const posY = ref(props.y)

function onClickOutside(e) {
  if (menuRef.value && !menuRef.value.contains(e.target)) {
    emit('close')
  }
}

function onKeydown(e) {
  if (e.key === 'Escape') emit('close')
}

async function handleAction(item) {
  if (item.disabled) return
  emit('close')
  if (item.action) await item.action()
}

onMounted(async () => {
  await nextTick()
  // Reposition if menu would overflow the viewport
  if (menuRef.value) {
    const rect = menuRef.value.getBoundingClientRect()
    if (rect.right > window.innerWidth) {
      posX.value = window.innerWidth - rect.width - 8
    }
    if (rect.bottom > window.innerHeight) {
      posY.value = window.innerHeight - rect.height - 8
    }
  }
  document.addEventListener('mousedown', onClickOutside)
  document.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  document.removeEventListener('mousedown', onClickOutside)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div
      ref="menuRef"
      class="fixed z-50 min-w-[160px] bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg shadow-2xl py-1 overflow-hidden"
      :style="{ left: posX + 'px', top: posY + 'px' }"
    >
      <template v-for="(item, i) in items" :key="i">
        <hr v-if="item.separator" class="my-1 border-[var(--border)]" />
        <button
          v-else
          class="flex items-center gap-2.5 w-full px-3 py-1.5 text-sm text-left transition-colors"
          :class="[
            item.disabled
              ? 'opacity-40 cursor-not-allowed'
              : item.variant === 'danger'
                ? 'hover:bg-red-900/30 text-[var(--text-primary)]'
                : 'hover:bg-[var(--bg-hover)] text-[var(--text-primary)]',
          ]"
          :disabled="item.disabled"
          @click="handleAction(item)"
        >
          <component :is="item.icon" v-if="item.icon" class="w-4 h-4 flex-shrink-0" :class="item.variant === 'danger' ? 'text-[var(--danger)]' : 'text-[var(--text-secondary)]'" />
          <span>{{ item.label }}</span>
        </button>
      </template>
    </div>
  </Teleport>
</template>
