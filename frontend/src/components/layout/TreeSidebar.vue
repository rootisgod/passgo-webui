<script setup>
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import StatusDot from '../shared/StatusDot.vue'
import ContextMenu from '../shared/ContextMenu.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import CloneVmModal from '../modals/CloneVmModal.vue'
import GroupNameModal from '../modals/GroupNameModal.vue'
import MoveToGroupModal from '../modals/MoveToGroupModal.vue'
import { Monitor, ChevronDown, ChevronRight, FileCode, Settings, Loader2, Play, Square, Pause, Copy, Trash2, RotateCcw, CheckSquare, Folder, FolderOpen, FolderPlus, Pencil, ArrowRight, Plus, Layers, TerminalSquare, Clock, KeyRound, ScrollText, Bell, LayoutTemplate, Tag, Network } from 'lucide-vue-next'
import { ref, computed, markRaw } from 'vue'

const store = useVmStore()
const toasts = useToastStore()
const expanded = ref(true)
const templatesExpanded = ref(true)
const selectionMode = ref(false)

// Context menu state
const contextMenu = ref(null)
const confirmAction = ref(null)
const cloneVmName = ref(null)

// Group modals
const groupModal = ref(null) // { mode, initialName, onConfirm }
const moveToGroupVm = ref(null) // vm name
const bulkMoveToGroup = ref(false)

// Bulk selection
const selectedStartableVms = computed(() => store.selectedVmObjects.filter(vm => (vm.state === 'Stopped' || vm.state === 'Suspended') && !store.isTemplate(vm.name)))
const selectedDeletableVms = computed(() => store.selectedVmObjects.filter(vm => vm.state !== 'Deleted' && !store.isTemplate(vm.name)))
const hasSelectedStopped = computed(() => selectedStartableVms.value.length > 0)
const hasSelectedRunning = computed(() => store.selectedVmObjects.some(vm => vm.state === 'Running' || vm.state === 'Suspended'))
const hasSelectedNonDeleted = computed(() => store.selectedVmObjects.some(vm => vm.state !== 'Deleted'))
const hasSelectedDeletable = computed(() => selectedDeletableVms.value.length > 0)
const allSelected = computed(() => store.vms.length > 0 && store.selectedVms.length === store.vms.length)
const allGroupsExpanded = computed(() => store.groups.length > 0 && store.groups.every(g => store.expandedGroups[g]))

function toggleSelectionMode() {
  selectionMode.value = !selectionMode.value
  if (selectionMode.value) {
    store.expandAllGroups()
  } else {
    store.clearSelection()
  }
}

function toggleSelectAll() {
  if (allSelected.value) {
    store.clearSelection()
  } else {
    store.selectAllVms()
  }
}

async function bulkAction(fn, label, names = [...store.selectedVms]) {
  if (names.length === 0) return
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
  const names = selectedStartableVms.value.map(vm => vm.name)
  bulkAction(name => api.startVM(name), 'Started', names)
}

function bulkStop() {
  bulkAction(name => api.stopVM(name), 'Stopped')
}

function bulkDelete() {
  const names = selectedDeletableVms.value.map(vm => vm.name)
  const count = names.length
  if (count === 0) return
  confirmAction.value = {
    message: `Delete ${count} VM${count !== 1 ? 's' : ''}?`,
    fn: () => bulkAction(name => api.deleteVM(name), 'Deleted', names),
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

async function setTemplate(vm, template) {
  try {
    await api.setVMTemplate(vm.name, template)
    toasts.success(template ? `"${vm.name}" marked as template` : `"${vm.name}" template tag removed`)
    store.fetchTemplates()
  } catch (e) { toasts.error(e.message) }
}

function addSeparator(items) {
  if (items.length > 0 && !items[items.length - 1].separator) {
    items.push({ separator: true })
  }
}

// Group summary badge
function groupBadge(groupName) {
  const s = store.groupSummary(groupName)
  if (s.total === 0) return ''
  const parts = []
  if (s.running) parts.push(`${s.running}R`)
  if (s.stopped) parts.push(`${s.stopped}S`)
  if (s.total - s.running - s.stopped > 0) parts.push(`${s.total - s.running - s.stopped}O`)
  return parts.join(' ')
}

// VM context menu
function openContextMenu(event, vm) {
  store.selectNode(vm.name)
  const isRunning = vm.state === 'Running'
  const isStopped = vm.state === 'Stopped'
  const isSuspended = vm.state === 'Suspended'
  const isDeleted = vm.state === 'Deleted'
  const isTemplate = store.isTemplate(vm.name)

  const items = []

  if (!isRunning && !isDeleted) {
    items.push({
      label: 'Start',
      icon: markRaw(Play),
      disabled: isTemplate,
      action: () => action(() => api.startVM(vm.name), `${vm.name} started`),
    })
  }
  if (isRunning || isSuspended) {
    items.push({ label: 'Stop', icon: markRaw(Square), action: () => action(() => api.stopVM(vm.name), `${vm.name} stopped`) })
  }
  if (isRunning) {
    items.push({ label: 'Suspend', icon: markRaw(Pause), action: () => action(() => api.suspendVM(vm.name), `${vm.name} suspended`) })
  }
  if (isStopped) {
    items.push({ label: isTemplate ? 'Clone Template' : 'Clone', icon: markRaw(Copy), action: () => { cloneVmName.value = vm.name } })
  }
  if (isDeleted) {
    items.push({ label: 'Recover', icon: markRaw(RotateCcw), action: () => action(() => api.recoverVM(vm.name), `${vm.name} recovered`) })
  }

  if (!isDeleted) {
    addSeparator(items)
    items.push({
      label: isTemplate ? 'Remove Template Tag' : 'Mark as Template',
      icon: markRaw(Tag),
      disabled: !isStopped,
      action: () => setTemplate(vm, !isTemplate),
    })
  }

  // Move to group
  if (!isDeleted && store.groups.length > 0) {
    items.push({ label: 'Move to Group...', icon: markRaw(ArrowRight), action: () => { moveToGroupVm.value = vm.name } })
  }

  if (!isDeleted) {
    addSeparator(items)
    items.push({
      label: 'Delete', icon: markRaw(Trash2), variant: 'danger',
      disabled: isTemplate,
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

// Group context menu
function openGroupContextMenu(event, groupName) {
  event.preventDefault()
  const groupVms = store.groupedVms(groupName)
  const hasRunning = groupVms.some(vm => vm.state === 'Running' || vm.state === 'Suspended')
  const startable = groupVms.filter(vm => (vm.state === 'Stopped' || vm.state === 'Suspended') && !store.isTemplate(vm.name))
  const deletable = groupVms.filter(vm => vm.state !== 'Deleted' && !store.isTemplate(vm.name))

  const items = []
  if (startable.length > 0) {
    items.push({
      label: 'Start All', icon: markRaw(Play),
      action: async () => {
        const results = await Promise.allSettled(startable.map(vm => api.startVM(vm.name)))
        const failed = results.filter(r => r.status === 'rejected')
        if (failed.length) toasts.error(`${failed.length} failed to start`)
        else toasts.success(`Started ${startable.length} VMs in ${groupName}`)
        store.fetchVMs()
      },
    })
  }
  if (hasRunning) {
    items.push({
      label: 'Stop All', icon: markRaw(Square),
      action: async () => {
        const running = groupVms.filter(vm => vm.state === 'Running' || vm.state === 'Suspended')
        const results = await Promise.allSettled(running.map(vm => api.stopVM(vm.name)))
        const failed = results.filter(r => r.status === 'rejected')
        if (failed.length) toasts.error(`${failed.length} failed to stop`)
        else toasts.success(`Stopped ${running.length} VMs in ${groupName}`)
        store.fetchVMs()
      },
    })
  }
  if (deletable.length > 0) {
    items.push({
      label: 'Delete All VMs', icon: markRaw(Trash2), variant: 'danger',
      action: () => {
        const count = deletable.length
        confirmAction.value = {
          message: `Delete all ${count} VM${count !== 1 ? 's' : ''} in "${groupName}"?`,
          fn: async () => {
            const results = await Promise.allSettled(deletable.map(vm => api.deleteVM(vm.name)))
            const failed = results.filter(r => r.status === 'rejected')
            if (failed.length) toasts.error(`${failed.length} failed to delete`)
            else toasts.success(`Deleted ${count} VMs in ${groupName}`)
            store.fetchVMs()
          },
        }
      },
    })
  }
  if (items.length > 0) items.push({ separator: true })
  items.push({
    label: 'Rename Group', icon: markRaw(Pencil),
    action: () => {
      groupModal.value = {
        mode: 'rename',
        initialName: groupName,
        onConfirm: async (newName) => {
          try {
            await api.renameGroup(groupName, newName)
            toasts.success(`Group renamed to "${newName}"`)
            store.fetchGroups()
          } catch (e) { toasts.error(e.message) }
        },
      }
    },
  })
  items.push({
    label: 'Delete Group', icon: markRaw(Trash2), variant: 'danger',
    action: () => {
      confirmAction.value = {
        message: `Delete group "${groupName}"? VMs will become ungrouped.`,
        fn: async () => {
          try {
            await api.deleteGroup(groupName)
            toasts.success(`Group "${groupName}" deleted`)
            store.fetchGroups()
          } catch (e) { toasts.error(e.message) }
        },
      }
    },
  })

  contextMenu.value = { x: event.clientX, y: event.clientY, items }
}

// Host context menu (for creating groups)
function openHostContextMenu(event) {
  contextMenu.value = {
    x: event.clientX,
    y: event.clientY,
    items: [{
      label: 'New Group', icon: markRaw(FolderPlus),
      action: () => {
        groupModal.value = {
          mode: 'create',
          initialName: '',
          onConfirm: async (name) => {
            try {
              await api.createGroup(name)
              toasts.success(`Group "${name}" created`)
              store.expandedGroups[name] = true
              store.fetchGroups()
            } catch (e) { toasts.error(e.message) }
          },
        }
      },
    }],
  }
}

async function handleMoveToGroup(groupName) {
  if (bulkMoveToGroup.value) {
    bulkMoveToGroup.value = false
    const names = [...store.selectedVms]
    const results = await Promise.allSettled(names.map(vm => api.assignVmGroup(vm, groupName)))
    const failed = results.filter(r => r.status === 'rejected')
    if (failed.length) {
      toasts.error(`${failed.length} of ${names.length} failed to move`)
    } else {
      toasts.success(groupName ? `Moved ${names.length} VM${names.length !== 1 ? 's' : ''} to "${groupName}"` : `Ungrouped ${names.length} VM${names.length !== 1 ? 's' : ''}`)
    }
    store.clearSelection()
    store.fetchGroups()
    return
  }
  const vmName = moveToGroupVm.value
  moveToGroupVm.value = null
  try {
    await api.assignVmGroup(vmName, groupName)
    toasts.success(groupName ? `Moved "${vmName}" to "${groupName}"` : `"${vmName}" ungrouped`)
    store.fetchGroups()
  } catch (e) { toasts.error(e.message) }
}

function handleGroupModalConfirm(name) {
  const fn = groupModal.value?.onConfirm
  groupModal.value = null
  if (fn) fn(name)
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

      <!-- Ansible -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__ansible__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__ansible__')"
      >
        <TerminalSquare class="w-4 h-4" />
        <span class="text-sm">Ansible</span>
      </div>

      <!-- Profiles -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__profiles__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__profiles__')"
      >
        <Layers class="w-4 h-4" />
        <span class="text-sm">Profiles</span>
      </div>

      <!-- Schedules -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__schedules__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__schedules__')"
      >
        <Clock class="w-4 h-4" />
        <span class="text-sm">Schedules</span>
      </div>

      <!-- Proxies -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__proxy-rules__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__proxy-rules__')"
      >
        <Network class="w-4 h-4" />
        <span class="text-sm">Proxies</span>
      </div>

      <!-- Webhooks -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__webhooks__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__webhooks__')"
      >
        <Bell class="w-4 h-4" />
        <span class="text-sm">Webhooks</span>
      </div>

      <!-- API Tokens -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__api-tokens__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__api-tokens__')"
      >
        <KeyRound class="w-4 h-4" />
        <span class="text-sm">API Tokens</span>
      </div>

      <!-- Event Log -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__events__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__events__')"
      >
        <ScrollText class="w-4 h-4" />
        <span class="text-sm">Event Log</span>
      </div>

      <!-- Settings -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === '__settings__' ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
        @click="store.selectNode('__settings__')"
      >
        <Settings class="w-4 h-4" />
        <span class="text-sm">Settings</span>
      </div>

      <hr class="my-1.5 border-[var(--border)]" />

      <!-- Host node -->
      <div
        class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors"
        :class="store.selectedNode === null ? 'bg-[var(--accent)]/20 text-[var(--accent)]' : 'hover:bg-[var(--bg-hover)]'"
        @click="selectHost"
        @contextmenu.prevent="openHostContextMenu"
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

      <!-- Select all toggle + expand/collapse all -->
      <div v-if="expanded && (selectionMode || store.groups.length > 0)" class="ml-4 px-2 py-1 flex items-center gap-3">
        <button
          v-if="selectionMode"
          class="text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] transition-colors"
          @click="toggleSelectAll"
        >
          {{ allSelected ? 'Deselect All' : 'Select All' }}
        </button>
        <button
          v-if="store.groups.length > 0"
          class="text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] transition-colors"
          @click="allGroupsExpanded ? store.collapseAllGroups() : store.expandAllGroups()"
        >
          {{ allGroupsExpanded ? 'Collapse All' : 'Expand All' }}
        </button>
      </div>

      <!-- Launching VMs -->
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

      <div v-show="expanded" class="ml-4">
        <!-- Template index -->
        <div v-if="store.templateVms.length > 0" class="mb-1">
          <div
            class="flex items-center gap-1.5 px-2 py-1 rounded cursor-pointer transition-colors hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]"
            @click="templatesExpanded = !templatesExpanded"
          >
            <ChevronDown v-if="templatesExpanded" class="w-3 h-3 flex-shrink-0" />
            <ChevronRight v-else class="w-3 h-3 flex-shrink-0" />
            <LayoutTemplate class="w-3.5 h-3.5 flex-shrink-0 text-[var(--warning)]" />
            <span class="text-sm truncate flex-1">Templates</span>
            <span class="text-[10px] text-[var(--muted)] whitespace-nowrap">{{ store.templateVms.length }}</span>
          </div>
          <div v-show="templatesExpanded" class="ml-4">
            <div
              v-for="vm in store.templateVms"
              :key="'template-' + vm.name"
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
              <span class="truncate flex-1">{{ vm.name }}</span>
              <button
                class="w-5 h-5 flex items-center justify-center rounded transition-colors disabled:opacity-35 disabled:cursor-not-allowed"
                :class="vm.state === 'Stopped' ? 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--accent)]' : 'text-[var(--muted)]'"
                :disabled="vm.state !== 'Stopped'"
                :title="vm.state === 'Stopped' ? 'Clone template' : 'Stop VM to clone template'"
                @click.stop="cloneVmName = vm.name"
              >
                <Copy class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>

        <!-- New Group button -->
        <button
          class="flex items-center gap-1.5 px-2 py-1 text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] transition-colors w-full"
          @click="groupModal = { mode: 'create', initialName: '', onConfirm: async (name) => { try { await api.createGroup(name); toasts.success(`Group '${name}' created`); store.expandedGroups[name] = true; store.fetchGroups() } catch(e) { toasts.error(e.message) } } }"
        >
          <Plus class="w-3 h-3" />
          <span>New Group</span>
        </button>

        <!-- Group folders -->
        <div v-for="group in store.groups" :key="'group-' + group">
          <!-- Group header -->
          <div
            class="flex items-center gap-1.5 px-2 py-1 rounded cursor-pointer transition-colors hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]"
            @click="store.toggleGroupExpanded(group)"
            @contextmenu.prevent="openGroupContextMenu($event, group)"
          >
            <ChevronDown v-if="store.expandedGroups[group]" class="w-3 h-3 flex-shrink-0" />
            <ChevronRight v-else class="w-3 h-3 flex-shrink-0" />
            <FolderOpen v-if="store.expandedGroups[group]" class="w-3.5 h-3.5 flex-shrink-0" />
            <Folder v-else class="w-3.5 h-3.5 flex-shrink-0" />
            <span class="text-sm truncate flex-1">{{ group }}</span>
            <span class="text-[10px] text-[var(--muted)] whitespace-nowrap">{{ groupBadge(group) }}</span>
          </div>
          <!-- VMs in group -->
          <div v-show="store.expandedGroups[group]" class="ml-4">
            <div
              v-for="vm in store.groupedVms(group)"
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
              <LayoutTemplate v-if="store.isTemplate(vm.name)" class="w-3 h-3 text-[var(--warning)] flex-shrink-0" title="Template" />
              <span class="truncate">{{ vm.name }}</span>
            </div>
            <div v-if="store.groupedVms(group).length === 0" class="px-2 py-1 text-xs text-[var(--muted)] italic">
              Empty
            </div>
          </div>
        </div>

        <!-- Ungrouped VMs -->
        <TransitionGroup name="list" tag="div">
          <div
            v-for="vm in store.ungroupedVms"
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
            <LayoutTemplate v-if="store.isTemplate(vm.name)" class="w-3 h-3 text-[var(--warning)] flex-shrink-0" title="Template" />
            <span class="truncate">{{ vm.name }}</span>
          </div>
        </TransitionGroup>
      </div>

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

    <GroupNameModal
      v-if="groupModal"
      :mode="groupModal.mode"
      :initial-name="groupModal.initialName"
      @confirm="handleGroupModalConfirm"
      @cancel="groupModal = null"
    />

    <MoveToGroupModal
      v-if="moveToGroupVm || bulkMoveToGroup"
      :vm-name="moveToGroupVm || ''"
      :vm-names="bulkMoveToGroup ? store.selectedVms : []"
      :current-group="moveToGroupVm ? (store.vmGroups[moveToGroupVm] || '') : ''"
      @confirm="handleMoveToGroup"
      @cancel="moveToGroupVm = null; bulkMoveToGroup = false"
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
          v-if="hasSelectedNonDeleted && store.groups.length > 0"
          class="flex items-center gap-1.5 px-2 py-1 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
          @click="bulkMoveToGroup = true"
        >
          <ArrowRight class="w-3 h-3" /> Move to Group
        </button>
        <button
          v-if="hasSelectedDeletable"
          class="flex items-center gap-1.5 px-2 py-1 text-xs rounded bg-red-900/30 hover:bg-[var(--danger)] text-red-300 hover:text-white transition-colors"
          @click="bulkDelete"
        >
          <Trash2 class="w-3 h-3" /> Delete
        </button>
      </div>
    </div>
  </aside>
</template>
