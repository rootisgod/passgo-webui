<script setup>
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import StatusDot from '../shared/StatusDot.vue'
import ContextMenu from '../shared/ContextMenu.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import CloneVmModal from '../modals/CloneVmModal.vue'
import { Monitor, ChevronDown, ChevronRight, FileCode, Loader2, Play, Square, Pause, Copy, Trash2, RotateCcw, CheckSquare, Square as SquareIcon } from 'lucide-vue-next'
import { ref, computed, markRaw } from 'vue'

const store = useVmStore()
const toasts = useToastStore()
const expanded = ref(true)
const selectionMode = ref(false)

// Context menu state
const contextMenu = ref(null) // { x, y, items }
const confirmAction = ref(null)
const cloneVmName = ref(null)

// Bulk selection
const hasSelectedStopped = computed(() => store.selectedVmObjects.some(vm => vm.state === 'Stopped' || vm.state === 'Suspended'))
const hasSelectedRunning = computed(() => store.selectedVmObjects.some(vm => vm.state === 'Running' || vm.state === 'Suspended'))
const hasSelectedNonDeleted = computed(() => store.selectedVmObjects.some(vm => vm.state !== 'Deleted'))
const allSelected = computed(() => store.vms.length > 0 && store.selectedVms.length === store.vms.length)

function toggleSelectionMode() {
  selectionMode.value = !selectionMode.value
  if (!selectionMode.value) store.clearSelection()
}

function toggleSelectAll() {
  if (allSelected.value) {
    store.clearSelection()
  } else {
    store.selectAllVms()
  }
}

async function bulkAction(fn, label) {
  const names = [...store.selectedVms]
  const results = await Promise.allSettled(names.map(fn))
  const failed = results.filter(r => r.status === 'rejected')
  if (failed.length) {
    toasts.error(`${failed.length} of ${names.length} failed`)
  } else {
    toasts.success(`${label} ${names.length} VM${names.length !== 1 ? 's' : ''}`)
  }
  store.clearSelection()
  store.fetchVMs()
}

function bulkStart() {
  bulkAction(name => api.startVM(name), 'Started')
}

function bulkStop() {
  bulkAction(name => api.stopVM(name), 'Stopped')
}

function bulkDelete() {
  const count = store.selectedVms.length
  confirmAction.value = {
    message: `Delete ${count} VM${count !== 1 ? 's' : ''}?`,
    fn: () => bulkAction(name => api.deleteVM(name), 'Deleted'),
  }
}

function selectHost() {
  store.selectNode(null)
}

function selectVM(name) {
  store.selectNode(name)
}

async function action(fn, msg) {
  try {
    await fn()
    toasts.success(msg)
    store.fetchVMs()
  } catch (e) { toasts.error(e.message) }
}

function openContextMenu(event, vm) {
  store.selectNode(vm.name)
  const isRunning = vm.state === 'Running'
  const isStopped = vm.state === 'Stopped'
  const isSuspended = vm.state === 'Suspended'
  const isDeleted = vm.state === 'Deleted'

  const items = []

  if (!isRunning) {
    items.push({ label: 'Start', icon: markRaw(Play), action: () => action(() => api.startVM(vm.name), `${vm.name} started`) })
  }
  if (isRunning || isSuspended) {
    items.push({ label: 'Stop', icon: markRaw(Square), action: () => action(() => api.stopVM(vm.name), `${vm.name} stopped`) })
  }
  if (isRunning) {
    items.push({ label: 'Suspend', icon: markRaw(Pause), action: () => action(() => api.suspendVM(vm.name), `${vm.name} suspended`) })
  }
  if (isStopped) {
    items.push({ label: 'Clone', icon: markRaw(Copy), action: () => { cloneVmName.value = vm.name } })
  }
  if (isDeleted) {
    items.push({ label: 'Recover', icon: markRaw(RotateCcw), action: () => action(() => api.recoverVM(vm.name), `${vm.name} recovered`) })
  }
  if (!isDeleted) {
    items.push({ separator: true })
    items.push({
      label: 'Delete', icon: markRaw(Trash2), variant: 'danger',
      action: () => {
        confirmAction.value = {
          message: `Delete VM '${vm.name}'?`,
          fn: () => action(() => api.deleteVM(vm.name), `${vm.name} deleted`),
        }
      },
    })
  }

  contextMenu.value = { x: event.clientX, y: event.clientY, items }
}

async function executeConfirmed() {
  const fn = confirmAction.value?.fn
  confirmAction.value = null
  if (fn) await fn()
}
</script>

<template>
  <aside class="w-60 bg-[var(--bg-secondary)] border-r border-[var(--border)] flex-shrink-0 select-none flex flex-col">
    <div class="p-2 flex-1 overflow-y-auto">
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
        <span class="text-sm font-medium truncate flex-1">{{ store.hostname }}</span>
        <button
          v-if="store.vms.length > 0"
          class="w-4 h-4 flex items-center justify-center transition-colors"
          :class="selectionMode ? 'text-[var(--accent)]' : 'text-[var(--muted)] hover:text-[var(--text-secondary)]'"
          title="Toggle selection mode"
          @click.stop="toggleSelectionMode"
        >
          <CheckSquare class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- Select all toggle -->
      <div v-if="selectionMode && expanded" class="ml-4 px-2 py-1">
        <button
          class="text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] transition-colors"
          @click="toggleSelectAll"
        >
          {{ allSelected ? 'Deselect All' : 'Select All' }}
        </button>
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
          @contextmenu.prevent="openContextMenu($event, vm)"
        >
          <input
            v-if="selectionMode"
            type="checkbox"
            :checked="store.selectedVms.includes(vm.name)"
            class="w-3.5 h-3.5 rounded border-[var(--border)] bg-[var(--bg-primary)] text-[var(--accent)] focus:ring-0 focus:ring-offset-0 cursor-pointer flex-shrink-0"
            @click.stop="store.toggleVmSelection(vm.name)"
          />
          <StatusDot :state="vm.state" />
          <span class="truncate">{{ vm.name }}</span>
        </div>
      </TransitionGroup>

      <div v-if="store.vms.length === 0 && store.launchingCount === 0 && expanded" class="ml-8 py-2 text-xs text-[var(--text-secondary)]">
        No VMs
      </div>
    </div>

    <ContextMenu
      v-if="contextMenu"
      :x="contextMenu.x"
      :y="contextMenu.y"
      :items="contextMenu.items"
      @close="contextMenu = null"
    />

    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />

    <CloneVmModal
      v-if="cloneVmName"
      :vm-name="cloneVmName"
      @close="cloneVmName = null"
      @cloned="cloneVmName = null"
    />

    <!-- Bulk action bar -->
    <div
      v-if="store.selectedVms.length > 0"
      class="border-t border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 flex-shrink-0"
    >
      <div class="text-xs text-[var(--text-secondary)] mb-2">
        {{ store.selectedVms.length }} selected
      </div>
      <div class="flex items-center gap-2">
        <button
          v-if="hasSelectedStopped"
          class="flex items-center gap-1.5 px-2 py-1 text-xs rounded bg-green-900/30 hover:bg-[var(--success)] text-green-300 hover:text-white transition-colors"
          @click="bulkStart"
        >
          <Play class="w-3 h-3" /> Start
        </button>
        <button
          v-if="hasSelectedRunning"
          class="flex items-center gap-1.5 px-2 py-1 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
          @click="bulkStop"
        >
          <Square class="w-3 h-3" /> Stop
        </button>
        <button
          v-if="hasSelectedNonDeleted"
          class="flex items-center gap-1.5 px-2 py-1 text-xs rounded bg-red-900/30 hover:bg-[var(--danger)] text-red-300 hover:text-white transition-colors"
          @click="bulkDelete"
        >
          <Trash2 class="w-3 h-3" /> Delete
        </button>
      </div>
    </div>
  </aside>
</template>
