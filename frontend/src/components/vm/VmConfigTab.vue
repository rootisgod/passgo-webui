<script setup>
import { ref, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { getVM } from '../../api/client.js'

const store = useVmStore()
const rawInfo = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    const data = await getVM(store.selectedNode)
    rawInfo.value = JSON.stringify(data, null, 2)
  } catch (e) {
    rawInfo.value = 'Failed to load VM info: ' + e.message
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="p-6">
    <h3 class="text-lg font-semibold mb-4">Configuration</h3>
    <div class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-4 overflow-auto max-h-[calc(100vh-250px)]">
      <pre v-if="!loading" class="text-sm font-mono text-[var(--text-secondary)] whitespace-pre-wrap">{{ rawInfo }}</pre>
      <div v-else class="text-[var(--text-secondary)] text-sm">Loading...</div>
    </div>
  </div>
</template>
