<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import CreateSnapshotModal from '../modals/CreateSnapshotModal.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import CloneVmModal from '../modals/CloneVmModal.vue'
import { Plus, RotateCcw, Copy, Trash2, AlertTriangle, GitBranch } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const snapshots = ref([])
const loading = ref(false)

// Multipass doesn't expose which snapshot is "current" — a VM's live state is
// a divergent working copy, not a snapshot. We use two signals:
//
// 1. localStorage override: when the user restores or creates through the UI,
//    we record that as the confirmed current. Survives page reloads, imperfect
//    across browsers but good enough for a homelab.
// 2. Fallback heuristic: newest snapshot by `created` timestamp. Per multipass
//    docs — "a snapshot's parent is the snapshot that was last taken or
//    restored" — so the newest snapshot was created from the VM's current
//    branch point, making it the most likely "current" reference.
//
// The fallback is wrong only if you restored to something after the newest
// snapshot was taken and haven't re-snapshotted. Once you do anything via the
// UI, the override kicks in and corrects it.
const currentKey = computed(() => `passgo:current-snapshot:${store.selectedNode}`)
const overrideSnapshot = ref('')

// The effective "current" the UI highlights: explicit override if set,
// else newest by created timestamp.
const currentSnapshot = computed(() => {
  if (overrideSnapshot.value && snapshots.value.some(s => s.name === overrideSnapshot.value)) {
    return overrideSnapshot.value
  }
  if (!snapshots.value.length) return ''
  let newest = snapshots.value[0]
  for (const s of snapshots.value) {
    if (s.created && (!newest.created || s.created > newest.created)) {
      newest = s
    }
  }
  return newest.name
})

function readOverride() {
  try {
    overrideSnapshot.value = localStorage.getItem(currentKey.value) || ''
  } catch {
    overrideSnapshot.value = ''
  }
}

function writeOverride(name) {
  try {
    if (name) {
      localStorage.setItem(currentKey.value, name)
    } else {
      localStorage.removeItem(currentKey.value)
    }
  } catch {
    // private browsing, no-op
  }
  overrideSnapshot.value = name || ''
}

// Format an RFC3339 timestamp into a short relative time like "3m ago" or "2d ago".
// Falls back to the raw string for unparseable input.
function relativeTime(iso) {
  if (!iso) return ''
  const then = Date.parse(iso)
  if (isNaN(then)) return iso
  const diff = Math.max(0, Date.now() - then)
  const s = Math.floor(diff / 1000)
  if (s < 60) return 'just now'
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  const d = Math.floor(h / 24)
  if (d < 30) return `${d}d ago`
  return new Date(then).toLocaleDateString()
}

// Build a flat list with depth, parent-link info, and last-sibling flags so
// the template can draw proper tree connectors (vertical lines down the
// ancestor columns, horizontal stub to each node).
const snapshotTree = computed(() => {
  const list = snapshots.value
  if (!list.length) return []

  const children = {}
  const roots = []
  for (const s of list) {
    if (!s.parent) {
      roots.push(s)
    } else {
      (children[s.parent] ||= []).push(s)
    }
  }
  // Sort siblings alphabetically so the tree renders deterministically.
  roots.sort((a, b) => a.name.localeCompare(b.name))
  for (const k in children) children[k].sort((a, b) => a.name.localeCompare(b.name))

  const result = []
  function walk(node, depth, ancestorHasMore) {
    const kids = children[node.name] || []
    result.push({
      ...node,
      depth,
      hasChildren: kids.length > 0,
      // For each ancestor column, does that ancestor have more siblings below?
      // Used to draw a vertical continuation line in that column.
      ancestorHasMore: [...ancestorHasMore],
    })
    for (let i = 0; i < kids.length; i++) {
      const isLastSibling = i === kids.length - 1
      walk(kids[i], depth + 1, [...ancestorHasMore, !isLastSibling])
    }
  }
  for (let i = 0; i < roots.length; i++) {
    const isLastRoot = i === roots.length - 1
    walk(roots[i], 0, [!isLastRoot])
  }
  return result
})

const showCreateModal = ref(false)
const cloneSnapshot = ref(null)
const confirmAction = ref(null)

const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')

async function loadSnapshots() {
  loading.value = true
  try {
    const data = await api.listSnapshots(store.selectedNode)
    snapshots.value = Array.isArray(data) ? data : []
    // If the override points at a deleted snapshot, clear it so the fallback
    // heuristic takes over.
    if (overrideSnapshot.value && !snapshots.value.some(s => s.name === overrideSnapshot.value)) {
      writeOverride('')
    }
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
        writeOverride(snap)
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
        if (overrideSnapshot.value === snap) writeOverride('')
        loadSnapshots()
        store.fetchVMs()
      } catch (e) { toasts.error(e.message) }
    },
  }
}

function onCreated(newName) {
  // A freshly-created snapshot captures the current working state, so it
  // becomes the current marker until a restore moves away from it.
  if (newName) writeOverride(newName)
  loadSnapshots()
}

async function executeConfirmed() {
  const fn = confirmAction.value?.fn
  confirmAction.value = null
  if (fn) await fn()
}

// Re-read current marker when the selected VM changes (tab navigation, VM switch)
watch(() => store.selectedNode, () => {
  readOverride()
  loadSnapshots()
})

onMounted(() => {
  readOverride()
  loadSnapshots()
})
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

    <!-- Explainer: multipass doesn't track "current" explicitly, so we default
         to the newest snapshot (per the docs, "parent is the snapshot last
         taken or restored") and let in-UI actions override. -->
    <p v-if="snapshotTree.length > 0" class="text-xs text-[var(--text-secondary)] mb-2">
      <span class="inline-flex items-center gap-1">
        <span class="w-2 h-2 rounded-full bg-green-500 inline-block" />
        Current
      </span>
      defaults to the newest snapshot and updates when you restore or create through the UI. Restores done via CLI aren't detected.
    </p>

    <!-- Snapshot tree -->
    <div v-if="snapshotTree.length > 0" class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] overflow-hidden">
      <div
        v-for="snap in snapshotTree"
        :key="snap.name"
        class="flex items-center px-4 py-2.5 border-b border-[var(--border)] last:border-b-0 text-sm transition-colors"
        :class="snap.name === currentSnapshot
          ? 'bg-green-900/25 hover:bg-green-900/35 border-l-4 border-l-green-500 pl-3'
          : 'hover:bg-[var(--bg-hover)]'"
      >
        <!-- Tree: one fixed-width column per depth level, drawing connector lines. -->
        <div class="flex items-stretch flex-1 min-w-0">
          <!-- Ancestor columns: vertical continuation line if that ancestor has more siblings below. -->
          <div
            v-for="(hasMore, i) in snap.ancestorHasMore.slice(0, snap.depth)"
            :key="i"
            class="relative flex-shrink-0"
            style="width: 24px"
          >
            <span
              v-if="hasMore"
              class="absolute top-0 bottom-0 left-1/2 w-px bg-[var(--border)]"
            />
          </div>

          <!-- Connector for the node itself: elbow from parent + horizontal stub. -->
          <div
            v-if="snap.depth > 0"
            class="relative flex-shrink-0"
            style="width: 24px"
          >
            <!-- vertical half: from top to middle if not last, full height otherwise -->
            <span
              class="absolute top-0 left-1/2 w-px bg-[var(--border)]"
              :class="snap.ancestorHasMore[snap.depth] ? 'bottom-0' : 'h-1/2'"
            />
            <!-- horizontal stub at vertical midpoint -->
            <span class="absolute left-1/2 right-0 h-px top-1/2 bg-[var(--border)]" />
          </div>

          <div class="flex items-center gap-2 min-w-0 flex-1">
            <GitBranch
              class="w-3.5 h-3.5 flex-shrink-0"
              :class="snap.name === currentSnapshot ? 'text-green-400' : 'text-[var(--accent)]'"
            />
            <span class="font-mono truncate" :class="snap.name === currentSnapshot ? 'font-semibold' : ''">
              {{ snap.name }}
            </span>
            <span
              v-if="snap.name === currentSnapshot"
              class="inline-flex items-center gap-1 px-2 py-0.5 text-[10px] font-semibold rounded-full bg-green-500/20 text-green-300 border border-green-600/40 flex-shrink-0"
              title="Last restored to or created in this browser session"
            >
              <span class="w-1.5 h-1.5 rounded-full bg-green-400 inline-block" />
              Current
            </span>
          </div>
        </div>

        <!-- Created time + comment -->
        <div class="w-56 text-[var(--text-secondary)] truncate px-3 flex items-center gap-2">
          <span
            v-if="snap.created"
            class="text-xs tabular-nums whitespace-nowrap flex-shrink-0"
            :title="new Date(snap.created).toLocaleString()"
          >
            {{ relativeTime(snap.created) }}
          </span>
          <span v-if="snap.created && snap.comment" class="text-[var(--border)]">·</span>
          <span v-if="snap.comment" class="truncate">{{ snap.comment }}</span>
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
            class="p-1 rounded hover:bg-[var(--accent)] transition-colors"
            :disabled="isRunning"
            :class="isRunning ? 'opacity-40 cursor-not-allowed' : ''"
            title="Clone to this snapshot"
            @click="cloneSnapshot = snap.name"
          >
            <Copy class="w-4 h-4" />
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
      @created="onCreated"
    />
    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />

    <CloneVmModal
      v-if="cloneSnapshot"
      :vm-name="store.selectedNode"
      :snapshot-name="cloneSnapshot"
      @close="cloneSnapshot = null"
      @cloned="cloneSnapshot = null"
    />
  </div>
</template>
