<script setup>
import { ref, computed, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import CreateSnapshotModal from '../modals/CreateSnapshotModal.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import { Plus, RotateCcw, Trash2, AlertTriangle, GitBranch, CornerDownRight } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const snapshots = ref([])
const loading = ref(false)

// Build a flat list ordered by tree depth with a `depth` property
const snapshotTree = computed(() => {
  const list = snapshots.value
  if (!list.length) return []

  // Index by name
  const byName = {}
  for (const s of list) byName[s.name] = s

  // Find children for each node
  const children = {}
  const roots = []
  for (const s of list) {
    if (!s.parent) {
      roots.push(s)
    } else {
      if (!children[s.parent]) children[s.parent] = []
      children[s.parent].push(s)
    }
  }

  // DFS to produce flat ordered list with depth
  const result = []
  function walk(node, depth) {
    result.push({ ...node, depth })
    const kids = children[node.name] || []
    for (const kid of kids) {
      walk(kid, depth + 1)
    }
  }
  for (const root of roots) walk(root, 0)

  return result
})
const showCreateModal = ref(false)
const confirmAction = ref(null)

const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')

async function loadSnapshots() {
  loading.value = true
  try {
    const data = await api.listSnapshots(store.selectedNode)
    snapshots.value = Array.isArray(data) ? data : []
  } catch (e) {
    toasts.error('Failed to load snapshots: ' + e.message)
    snapshots.value = []
  } finally {
    loading.value = false
  }
}

async function restoreSnapshot(snap) {
  confirmAction.value = {
    message: `Restore snapshot '${snap}'? This is destructive and will overwrite the current VM state.`,
    fn: async () => {
      try {
        await api.restoreSnapshot(store.selectedNode, snap)
        toasts.success(`Snapshot '${snap}' restored`)
        loadSnapshots()
        store.fetchVMs()
      } catch (e) { toasts.error(e.message) }
    },
  }
}

async function deleteSnap(snap) {
  confirmAction.value = {
    message: `Delete snapshot '${snap}'?`,
    fn: async () => {
      try {
        await api.deleteSnapshot(store.selectedNode, snap)
        toasts.success(`Snapshot '${snap}' deleted`)
        loadSnapshots()
        store.fetchVMs()
      } catch (e) { toasts.error(e.message) }
    },
  }
}

async function executeConfirmed() {
  const fn = confirmAction.value?.fn
  confirmAction.value = null
  if (fn) await fn()
}

onMounted(loadSnapshots)
</script>

<template>
  <div class="p-6">
    <!-- Warning for running VMs -->
    <div v-if="isRunning" class="flex items-center gap-2 p-3 mb-4 rounded-lg bg-yellow-900/20 border border-yellow-800 text-yellow-300 text-sm">
      <AlertTriangle class="w-4 h-4 flex-shrink-0" />
      Stop the VM to manage snapshots
    </div>

    <div class="flex items-center justify-between mb-4">
      <h3 class="text-lg font-semibold">Snapshots</h3>
      <ActionButton
        label="Create Snapshot"
        :icon="Plus"
        variant="success"
        :disabled="isRunning"
        @click="showCreateModal = true"
      />
    </div>

    <!-- Snapshot tree -->
    <div v-if="snapshotTree.length > 0" class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] overflow-hidden">
      <div
        v-for="snap in snapshotTree"
        :key="snap.name"
        class="flex items-center px-4 py-2.5 border-b border-[var(--border)] last:border-b-0 hover:bg-[var(--bg-hover)] text-sm"
      >
        <!-- Indented name with tree connector -->
        <div class="flex items-center gap-1.5 flex-1 min-w-0" :style="{ paddingLeft: snap.depth * 24 + 'px' }">
          <CornerDownRight v-if="snap.depth > 0" class="w-3.5 h-3.5 text-[var(--border)] flex-shrink-0" />
          <GitBranch v-else class="w-3.5 h-3.5 text-[var(--accent)] flex-shrink-0" />
          <span class="font-mono truncate">{{ snap.name }}</span>
        </div>

        <!-- Comment -->
        <div class="w-48 text-[var(--text-secondary)] truncate px-3">
          {{ snap.comment || '' }}
        </div>

        <!-- Actions -->
        <div class="flex items-center gap-2 flex-shrink-0">
          <button
            class="p-1 rounded hover:bg-[var(--accent)] transition-colors"
            :disabled="isRunning"
            :class="isRunning ? 'opacity-40 cursor-not-allowed' : ''"
            title="Restore"
            @click="restoreSnapshot(snap.name)"
          >
            <RotateCcw class="w-4 h-4" />
          </button>
          <button
            class="p-1 rounded hover:bg-[var(--danger)] transition-colors"
            :disabled="isRunning"
            :class="isRunning ? 'opacity-40 cursor-not-allowed' : ''"
            title="Delete"
            @click="deleteSnap(snap.name)"
          >
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <div v-else-if="!loading" class="text-[var(--text-secondary)] text-sm">
      No snapshots
    </div>

    <CreateSnapshotModal
      v-if="showCreateModal"
      :vm-name="store.selectedNode"
      @close="showCreateModal = false"
      @created="loadSnapshots"
    />
    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />
  </div>
</template>
