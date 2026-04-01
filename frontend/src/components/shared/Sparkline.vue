<script setup>
import { computed } from 'vue'

const props = defineProps({
  data: { type: Array, default: () => [] },
  width: { type: Number, default: 200 },
  height: { type: Number, default: 32 },
  color: { type: String, default: 'var(--accent)' },
  max: { type: Number, default: null }, // fixed max, or auto-scale
})

const points = computed(() => {
  const d = props.data
  if (d.length < 2) return ''
  const max = props.max != null ? props.max : Math.max(...d, 1)
  const step = props.width / (d.length - 1)
  return d.map((v, i) => {
    const x = i * step
    const y = props.height - (v / max) * (props.height - 2) - 1
    return `${x},${y}`
  }).join(' ')
})

const fillPoints = computed(() => {
  if (!points.value) return ''
  const d = props.data
  const step = props.width / (d.length - 1)
  return `0,${props.height} ${points.value} ${(d.length - 1) * step},${props.height}`
})
</script>

<template>
  <svg :viewBox="`0 0 ${width} ${height}`" :height="height" class="block w-full" preserveAspectRatio="none" v-if="data.length >= 2">
    <polygon
      :points="fillPoints"
      :fill="color"
      fill-opacity="0.1"
    />
    <polyline
      :points="points"
      fill="none"
      :stroke="color"
      stroke-width="1.5"
      stroke-linejoin="round"
      stroke-linecap="round"
    />
  </svg>
  <div v-else class="flex items-center justify-center text-xs text-[var(--muted)] w-full" :style="{ height: height + 'px' }">
    Collecting data...
  </div>
</template>
