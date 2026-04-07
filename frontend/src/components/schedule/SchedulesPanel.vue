<script setup>
import { ref, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { Plus, Pencil, Trash2, Save, X, Play } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()

const DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const schedules = ref([])
const playbooks = ref([])
const history = ref([])
const loading = ref(true)
const editing = ref(null)
const saving = ref(false)

// Form state
const formId = ref('')
const formName = ref('')
const formEnabled = ref(true)
const formAction = ref('start')
const formTime = ref('08:00')
const formDays = ref([1, 2, 3, 4, 5])
const formTargetMode = ref('vms')
const formVMs = ref([])
const formGroup = ref('')
const formPlaybook = ref('')

async function fetchHistory() {
  try {
    const h = await api.getScheduleHistory()
    history.value = Array.isArray(h) ? h : []
  } catch { /* ignore */ }
}

onMounted(async () => {
  try {
    const [scheds, pbs] = await Promise.all([
      api.listSchedules().catch(() => []),
      api.listPlaybooks().catch(() => []),
    ])
    schedules.value = Array.isArray(scheds) ? scheds : []
    playbooks.value = Array.isArray(pbs) ? pbs : []
    await fetchHistory()
  } catch { /* ignore */ }
  loading.value = false
})

function formatTimestamp(ts) {
  try {
    const d = new Date(ts)
    return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch { return ts }
}

function resultClass(result) {
  switch (result) {
    case 'success': return 'text-[var(--success)]'
    case 'partial': return 'text-[var(--warning,orange)]'
    case 'failed': return 'text-[var(--danger)]'
    case 'no_targets': return 'text-[var(--text-secondary)]'
    default: return ''
  }
}

function startNew() {
  editing.value = '__new__'
  formId.value = ''
  formName.value = ''
  formEnabled.value = true
  formAction.value = 'start'
  formTime.value = '08:00'
  formDays.value = [1, 2, 3, 4, 5]
  formTargetMode.value = 'vms'
  formVMs.value = []
  formGroup.value = ''
  formPlaybook.value = ''
}

function startEdit(s) {
  editing.value = s.id
  formId.value = s.id
  formName.value = s.name
  formEnabled.value = s.enabled
  formAction.value = s.action
  formTime.value = s.time
  formDays.value = [...(s.days || [])]
  formTargetMode.value = s.group ? 'group' : 'vms'
  formVMs.value = [...(s.vms || [])]
  formGroup.value = s.group || ''
  formPlaybook.value = s.playbook || ''
}

function cancelEdit() {
  editing.value = null
}

function toggleDay(day) {
  const idx = formDays.value.indexOf(day)
  if (idx >= 0) {
    formDays.value.splice(idx, 1)
  } else {
    formDays.value.push(day)
  }
}

function setAllDays() {
  if (formDays.value.length === 7) {
    formDays.value = []
  } else {
    formDays.value = [0, 1, 2, 3, 4, 5, 6]
  }
}

function toggleVM(name) {
  const idx = formVMs.value.indexOf(name)
  if (idx >= 0) {
    formVMs.value.splice(idx, 1)
  } else {
    formVMs.value.push(name)
  }
}

async function saveSchedule() {
  saving.value = true
  try {
    const sched = {
      id: formId.value,
      name: formName.value,
      enabled: formEnabled.value,
      action: formAction.value,
      time: formTime.value,
      days: formDays.value,
      vms: formTargetMode.value === 'vms' ? formVMs.value : [],
      group: formTargetMode.value === 'group' ? formGroup.value : '',
      playbook: formAction.value === 'playbook' ? formPlaybook.value : '',
    }
    if (editing.value === '__new__') {
      await api.createSchedule(sched)
      toasts.success(`Schedule "${formName.value}" created`)
    } else {
      await api.updateSchedule(editing.value, sched)
      toasts.success(`Schedule "${formName.value}" updated`)
    }
    schedules.value = await api.listSchedules()
    editing.value = null
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}

async function deleteSchedule(id, name) {
  if (!confirm(`Delete schedule "${name}"?`)) return
  try {
    await api.deleteSchedule(id)
    toasts.success(`Schedule "${name}" deleted`)
    if (editing.value === id) editing.value = null
    schedules.value = await api.listSchedules()
  } catch (e) {
    toasts.error(e.message)
  }
}

async function runNow(s) {
  try {
    await api.runScheduleNow(s.id)
    toasts.success(`Schedule "${s.name}" executed`)
    await fetchHistory()
  } catch (e) {
    toasts.error(e.message)
  }
}

async function toggleEnabled(s) {
  try {
    await api.updateSchedule(s.id, { ...s, enabled: !s.enabled })
    schedules.value = await api.listSchedules()
  } catch (e) {
    toasts.error(e.message)
  }
}

function formatDays(days) {
  if (!days || days.length === 0) return 'No days'
  if (days.length === 7) return 'Every day'
  const weekdays = [1, 2, 3, 4, 5]
  const weekends = [0, 6]
  if (days.length === 5 && weekdays.every(d => days.includes(d))) return 'Weekdays'
  if (days.length === 2 && weekends.every(d => days.includes(d))) return 'Weekends'
  return [...days].sort((a, b) => a - b).map(d => DAY_NAMES[d]).join(', ')
}

function actionLabel(action) {
  switch (action) {
    case 'start': return 'Start'
    case 'stop': return 'Stop'
    case 'playbook': return 'Playbook'
    default: return action
  }
}

function scheduleSummary(s) {
  const parts = [actionLabel(s.action), s.time, formatDays(s.days)]
  if (s.group) parts.push(`[${s.group}]`)
  else if (s.vms?.length) parts.push(`${s.vms.length} VM${s.vms.length > 1 ? 's' : ''}`)
  if (s.action === 'playbook' && s.playbook) parts.push(s.playbook)
  return parts.join(' · ')
}

function canSave() {
  if (!formId.value || !formName.value || formDays.value.length === 0) return false
  if (formTargetMode.value === 'vms' && formVMs.value.length === 0) return false
  if (formTargetMode.value === 'group' && !formGroup.value) return false
  if (formAction.value === 'playbook' && !formPlaybook.value) return false
  return true
}
</script>

<template>
  <div class="h-full flex flex-col">
    <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
      <h2 class="text-lg font-semibold">Scheduled Operations</h2>
      <button
        @click="startNew"
        :disabled="editing === '__new__'"
        class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
      >
        <Plus class="w-3.5 h-3.5" />
        New Schedule
      </button>
    </div>

    <div v-if="loading" class="flex-1 flex items-center justify-center text-[var(--text-secondary)] text-sm">
      Loading...
    </div>

    <div v-else class="flex-1 overflow-y-auto">
      <!-- Empty state -->
      <div v-if="schedules.length === 0 && editing !== '__new__'" class="p-6 text-center text-[var(--text-secondary)] text-sm">
        <p>No scheduled operations yet.</p>
        <p class="mt-1">Schedules automate VM start/stop and playbook runs at specific times.</p>
      </div>

      <div class="divide-y divide-[var(--border)]">
        <!-- New schedule form -->
        <div v-if="editing === '__new__'" class="p-4 bg-[var(--bg-primary)]/50">
          <div class="space-y-3">
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">ID</label>
                <input v-model="formId" type="text" placeholder="e.g. stop-dev-vms"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Name</label>
                <input v-model="formName" type="text" placeholder="e.g. Stop dev VMs at night"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>
            </div>

            <!-- Action -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Action</label>
              <select v-model="formAction"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="start">Start VMs</option>
                <option value="stop">Stop VMs</option>
                <option value="playbook">Run Playbook</option>
              </select>
            </div>

            <!-- Time -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Time</label>
              <input v-model="formTime" type="time"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
            </div>

            <!-- Days -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Days</label>
              <div class="flex gap-1.5 items-center">
                <button v-for="(name, idx) in DAY_NAMES" :key="idx"
                  @click="toggleDay(idx)"
                  class="px-2 py-1 text-xs rounded transition-colors"
                  :class="formDays.includes(idx) ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                >{{ name }}</button>
                <button @click="setAllDays"
                  class="ml-2 px-2 py-1 text-xs rounded bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] transition-colors"
                >{{ formDays.length === 7 ? 'Clear' : 'All' }}</button>
              </div>
            </div>

            <!-- Target mode -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Target</label>
              <div class="flex gap-4 mb-2">
                <label class="flex items-center gap-1.5 text-sm text-[var(--text-primary)] cursor-pointer">
                  <input v-model="formTargetMode" type="radio" value="vms" class="accent-[var(--accent)]" />
                  Specific VMs
                </label>
                <label class="flex items-center gap-1.5 text-sm text-[var(--text-primary)] cursor-pointer">
                  <input v-model="formTargetMode" type="radio" value="group" class="accent-[var(--accent)]" />
                  VM Group
                </label>
              </div>

              <!-- VM checkboxes -->
              <div v-if="formTargetMode === 'vms'" class="max-h-32 overflow-y-auto bg-[var(--bg-primary)] border border-[var(--border)] rounded p-2 space-y-1">
                <div v-if="store.vms.length === 0" class="text-xs text-[var(--text-secondary)]">No VMs available</div>
                <label v-for="vm in store.vms" :key="vm.name" class="flex items-center gap-2 text-sm text-[var(--text-primary)] cursor-pointer">
                  <input type="checkbox" :checked="formVMs.includes(vm.name)" @change="toggleVM(vm.name)" class="accent-[var(--accent)]" />
                  {{ vm.name }}
                </label>
              </div>

              <!-- Group dropdown -->
              <select v-if="formTargetMode === 'group'" v-model="formGroup"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="">Select a group</option>
                <option v-for="g in store.groups" :key="g" :value="g">{{ g }}</option>
              </select>
            </div>

            <!-- Playbook (only for playbook action) -->
            <div v-if="formAction === 'playbook'">
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Playbook</label>
              <select v-model="formPlaybook"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="">Select a playbook</option>
                <option v-for="pb in playbooks" :key="pb.name" :value="pb.name">{{ pb.name }}</option>
              </select>
            </div>

            <!-- Enabled -->
            <label class="flex items-center gap-2 text-sm text-[var(--text-primary)] cursor-pointer">
              <input v-model="formEnabled" type="checkbox" class="accent-[var(--accent)]" />
              Enabled
            </label>

            <div class="flex justify-end gap-2 pt-1">
              <button @click="cancelEdit"
                class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors">
                <X class="w-3.5 h-3.5" /> Cancel
              </button>
              <button @click="saveSchedule" :disabled="!canSave() || saving"
                class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40">
                <Save class="w-3.5 h-3.5" /> Create
              </button>
            </div>
          </div>
        </div>

        <!-- Existing schedules -->
        <div v-for="s in schedules" :key="s.id">
          <!-- Edit mode -->
          <div v-if="editing === s.id" class="p-4 bg-[var(--bg-primary)]/50">
            <div class="space-y-3">
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Name</label>
                <input v-model="formName" type="text"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Action</label>
                <select v-model="formAction"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="start">Start VMs</option>
                  <option value="stop">Stop VMs</option>
                  <option value="playbook">Run Playbook</option>
                </select>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Time</label>
                <input v-model="formTime" type="time"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Days</label>
                <div class="flex gap-1.5 items-center">
                  <button v-for="(name, idx) in DAY_NAMES" :key="idx"
                    @click="toggleDay(idx)"
                    class="px-2 py-1 text-xs rounded transition-colors"
                    :class="formDays.includes(idx) ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                  >{{ name }}</button>
                  <button @click="setAllDays"
                    class="ml-2 px-2 py-1 text-xs rounded bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] transition-colors"
                  >{{ formDays.length === 7 ? 'Clear' : 'All' }}</button>
                </div>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Target</label>
                <div class="flex gap-4 mb-2">
                  <label class="flex items-center gap-1.5 text-sm text-[var(--text-primary)] cursor-pointer">
                    <input v-model="formTargetMode" type="radio" value="vms" class="accent-[var(--accent)]" />
                    Specific VMs
                  </label>
                  <label class="flex items-center gap-1.5 text-sm text-[var(--text-primary)] cursor-pointer">
                    <input v-model="formTargetMode" type="radio" value="group" class="accent-[var(--accent)]" />
                    VM Group
                  </label>
                </div>

                <div v-if="formTargetMode === 'vms'" class="max-h-32 overflow-y-auto bg-[var(--bg-primary)] border border-[var(--border)] rounded p-2 space-y-1">
                  <div v-if="store.vms.length === 0" class="text-xs text-[var(--text-secondary)]">No VMs available</div>
                  <label v-for="vm in store.vms" :key="vm.name" class="flex items-center gap-2 text-sm text-[var(--text-primary)] cursor-pointer">
                    <input type="checkbox" :checked="formVMs.includes(vm.name)" @change="toggleVM(vm.name)" class="accent-[var(--accent)]" />
                    {{ vm.name }}
                  </label>
                </div>

                <select v-if="formTargetMode === 'group'" v-model="formGroup"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="">Select a group</option>
                  <option v-for="g in store.groups" :key="g" :value="g">{{ g }}</option>
                </select>
              </div>

              <div v-if="formAction === 'playbook'">
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Playbook</label>
                <select v-model="formPlaybook"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="">Select a playbook</option>
                  <option v-for="pb in playbooks" :key="pb.name" :value="pb.name">{{ pb.name }}</option>
                </select>
              </div>

              <label class="flex items-center gap-2 text-sm text-[var(--text-primary)] cursor-pointer">
                <input v-model="formEnabled" type="checkbox" class="accent-[var(--accent)]" />
                Enabled
              </label>

              <div class="flex justify-end gap-2 pt-1">
                <button @click="cancelEdit"
                  class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors">
                  <X class="w-3.5 h-3.5" /> Cancel
                </button>
                <button @click="saveSchedule" :disabled="!canSave() || saving"
                  class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40">
                  <Save class="w-3.5 h-3.5" /> Save
                </button>
              </div>
            </div>
          </div>

          <!-- View mode -->
          <div v-else class="flex items-center gap-3 px-4 py-3 hover:bg-[var(--bg-hover)] transition-colors">
            <button @click="toggleEnabled(s)"
              class="w-8 h-4 rounded-full transition-colors relative flex-shrink-0"
              :class="s.enabled ? 'bg-[var(--accent)]' : 'bg-[var(--border)]'"
              :title="s.enabled ? 'Disable' : 'Enable'"
            >
              <span class="absolute top-0.5 w-3 h-3 rounded-full bg-white transition-transform"
                :class="s.enabled ? 'left-4' : 'left-0.5'" />
            </button>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium" :class="s.enabled ? 'text-[var(--text-primary)]' : 'text-[var(--text-secondary)]'">{{ s.name }}</div>
              <div class="text-xs text-[var(--text-secondary)] truncate">{{ scheduleSummary(s) }}</div>
            </div>
            <button @click="runNow(s)" title="Run Now"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--success)]">
              <Play class="w-3.5 h-3.5" />
            </button>
            <button @click="startEdit(s)"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)]">
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button @click="deleteSchedule(s.id, s.name)"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--danger)]">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      <!-- Run History -->
      <div v-if="history.length > 0" class="border-t border-[var(--border)]">
        <div class="px-4 py-3">
          <h3 class="text-sm font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-3">Run History</h3>
          <div class="space-y-1.5">
            <div v-for="(h, i) in history" :key="i"
              class="flex items-center gap-3 px-3 py-2 rounded bg-[var(--bg-primary)] text-xs">
              <span class="text-[var(--text-secondary)] flex-shrink-0 w-32">{{ formatTimestamp(h.timestamp) }}</span>
              <span class="font-medium text-[var(--text-primary)] flex-shrink-0">{{ h.schedule_name }}</span>
              <span class="text-[var(--text-secondary)]">{{ actionLabel(h.action) }}</span>
              <span v-if="h.targets?.length" class="text-[var(--text-secondary)] truncate">{{ h.targets.join(', ') }}</span>
              <span class="ml-auto flex-shrink-0 font-medium" :class="resultClass(h.result)">{{ h.result }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
