<script setup>
import { useVmStore } from '../../stores/vmStore.js'
import StatusDot from '../shared/StatusDot.vue'
import { Monitor, ChevronDown, ChevronRight, FileCode, Loader2 } from 'lucide-vue-next'
import { ref } from 'vue'

const store = useVmStore()
const expanded = ref(true)

function selectHost() {
  store.selectNode(null)
}

function selectVM(name) {
  store.selectNode(name)
}
</script>

<template>
  <aside class="w-60 bg-[var(--bg-secondary)] border-r border-[var(--border)] overflow-y-auto flex-shrink-0">
    <div class="p-2">
      <!-- Cloud-Init -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__cloud-init__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__cloud-init__')"
      >
        <FileCode class="w-4 h-4" />
        <span class="text-sm">Cloud-Init</span>
      </div>

      <hr class="my-1.5 border-[var(--border)]" />

      <!-- Host node -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === null ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)]'"
        @click="selectHost"
      >
        <button
          class="w-4 h-4 flex items-center justify-center"
          @click.stop="expanded = !expanded"
        >
          <ChevronDown v-if="expanded" class="w-3 h-3" />
          <ChevronRight v-else class="w-3 h-3" />
        </button>
        <Monitor class="w-4 h-4" />
        <span class="text-sm font-medium truncate">{{ store.hostname }}</span>
      </div>

      <!-- Launching VMs (only shown if not yet in the real VM list) -->
      <div v-show="expanded" class="ml-4">
        <div
          v-for="launch in store.activeLaunches"
          :key="'launch-' + launch.name"
          class="flex items-center gap-2 px-2 py-1 text-sm text-[var(--text-secondary)]"
        >
          <Loader2 class="w-3.5 h-3.5 animate-spin text-[var(--accent)]" />
          <span class="truncate opacity-70">{{ launch.name }}</span>
        </div>
      </div>

      <!-- VM nodes -->
      <TransitionGroup name="list" tag="div" v-show="expanded" class="ml-4">
        <div
          v-for="vm in store.vms"
          :key="vm.name"
          class="flex items-center gap-2 px-2 py-1 rounded cursor-pointer transition-colors text-sm"
          :class="store.selectedNode === vm.name ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
          @click="selectVM(vm.name)"
        >
          <StatusDot :state="vm.state" />
          <span class="truncate">{{ vm.name }}</span>
        </div>
      </TransitionGroup>

      <div v-if="store.vms.length === 0 && store.launchingCount === 0 && expanded" class="ml-8 py-2 text-xs text-[var(--text-secondary)]">
        No VMs
      </div>
    </div>
  </aside>
</template>
