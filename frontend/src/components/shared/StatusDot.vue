<script setup>
const props = defineProps({
  state: { type: String, default: 'Stopped' },
})

const colorMap = {
  Running: 'bg-[var(--success)]',
  Stopped: 'bg-[var(--muted)]',
  Suspended: 'bg-[var(--warning)]',
  Deleted: 'bg-[var(--danger)]',
  Starting: 'bg-[var(--success)] status-pulse',
  Suspending: 'bg-[var(--warning)] status-pulse',
  Creating: 'bg-[var(--accent)] status-pulse',
}
</script>

<template>
  <span
    class="inline-block w-2.5 h-2.5 rounded-full"
    :class="colorMap[state] || 'bg-[var(--muted)]'"
  />
</template>
