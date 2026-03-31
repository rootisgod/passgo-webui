<script setup>
import { ref, watch } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import VmSummaryTab from './VmSummaryTab.vue'
import VmConsoleTab from './VmConsoleTab.vue'
import VmSnapshotsTab from './VmSnapshotsTab.vue'
import VmMountsTab from './VmMountsTab.vue'
import VmConfigTab from './VmConfigTab.vue'

const store = useVmStore()
const activeTab = ref('summary')

const tabs = [
  { id: 'summary', label: 'Summary' },
  { id: 'console', label: 'Console' },
  { id: 'snapshots', label: 'Snapshots' },
  { id: 'mounts', label: 'Mounts' },
  { id: 'config', label: 'Config' },
]

// Reset tab when VM changes
watch(() => store.selectedNode, () => {
  activeTab.value = 'summary'
})
</script>

<template>
  <div class="flex flex-col h-full" v-if="store.selectedVm">
    <!-- Tab bar -->
    <div class="flex border-b border-[var(--border)] bg-[var(--bg-secondary)] px-4">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        class="px-4 py-2.5 text-sm transition-colors relative"
        :class="activeTab === tab.id
          ? 'text-[var(--accent)]'
          : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'"
        @click="activeTab = tab.id"
      >
        {{ tab.label }}
        <div
          v-if="activeTab === tab.id"
          class="absolute bottom-0 left-0 right-0 h-0.5 bg-[var(--accent)] tab-indicator"
        />
      </button>
    </div>

    <!-- Tab content -->
    <div class="flex-1 overflow-auto">
      <Transition name="fade" mode="out-in">
        <VmSummaryTab v-if="activeTab === 'summary'" :key="'summary-' + store.selectedNode" />
        <VmConsoleTab v-else-if="activeTab === 'console'" :key="'console-' + store.selectedNode" />
        <VmSnapshotsTab v-else-if="activeTab === 'snapshots'" :key="'snap-' + store.selectedNode" />
        <VmMountsTab v-else-if="activeTab === 'mounts'" :key="'mounts-' + store.selectedNode" />
        <VmConfigTab v-else-if="activeTab === 'config'" :key="'config-' + store.selectedNode" />
      </Transition>
    </div>
  </div>

  <div v-else class="flex items-center justify-center h-full text-[var(--text-secondary)]">
    Select a VM from the tree
  </div>
</template>
