<script setup>
import { ref, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import { Plus, Trash2 } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const mounts = ref([])
const loading = ref(false)
const showAddForm = ref(false)
const newSource = ref('')
const newTarget = ref('')
const confirmAction = ref(null)

async function loadMounts() {
  loading.value = true
  try {
    const data = await api.listMounts(store.selectedNode)
    mounts.value = Array.isArray(data) ? data : []
  } catch (e) {
    mounts.value = []
  } finally {
    loading.value = false
  }
}

async function addMount() {
  if (!newSource.value || !newTarget.value) return
  try {
    await api.addMount(store.selectedNode, newSource.value, newTarget.value)
    toasts.success('Mount added')
    newSource.value = ''
    newTarget.value = ''
    showAddForm.value = false
    loadMounts()
  } catch (e) { toasts.error(e.message) }
}

function confirmRemove(target) {
  confirmAction.value = {
    message: `Remove mount '${target}'?`,
    fn: async () => {
      try {
        await api.removeMount(store.selectedNode, target)
        toasts.success('Mount removed')
        loadMounts()
      } catch (e) { toasts.error(e.message) }
    },
  }
}

async function executeConfirmed() {
  const fn = confirmAction.value?.fn
  confirmAction.value = null
  if (fn) await fn()
}

onMounted(loadMounts)
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-lg font-semibold">Mounts</h3>
      <ActionButton label="Add Mount" :icon="Plus" variant="success" @click="showAddForm = !showAddForm" />
    </div>

    <!-- Add mount form -->
    <div v-if="showAddForm" class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-4 mb-4">
      <div class="grid grid-cols-2 gap-4 mb-3">
        <div>
          <label class="block text-xs text-[var(--text-secondary)] mb-1">Host path</label>
          <input
            v-model="newSource"
            type="text"
            placeholder="/home/user/shared"
            class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
          />
        </div>
        <div>
          <label class="block text-xs text-[var(--text-secondary)] mb-1">VM target path</label>
          <input
            v-model="newTarget"
            type="text"
            placeholder="/mnt/shared"
            class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
          />
        </div>
      </div>
      <div class="flex gap-2">
        <button
          @click="addMount"
          class="px-3 py-1.5 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors"
        >Mount</button>
        <button
          @click="showAddForm = false"
          class="px-3 py-1.5 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
        >Cancel</button>
      </div>
    </div>

    <!-- Mount table -->
    <div v-if="mounts.length > 0" class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] overflow-hidden">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-[var(--border)] text-[var(--text-secondary)]">
            <th class="text-left px-4 py-2.5 font-medium">Source Path</th>
            <th class="text-left px-4 py-2.5 font-medium">Target Path</th>
            <th class="text-left px-4 py-2.5 font-medium">UID Maps</th>
            <th class="text-left px-4 py-2.5 font-medium">GID Maps</th>
            <th class="text-right px-4 py-2.5 font-medium">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="mount in mounts"
            :key="mount.target_path"
            class="border-b border-[var(--border)] last:border-b-0 hover:bg-[var(--bg-hover)]"
          >
            <td class="px-4 py-2.5 font-mono text-xs">{{ mount.source_path }}</td>
            <td class="px-4 py-2.5 font-mono text-xs">{{ mount.target_path }}</td>
            <td class="px-4 py-2.5 text-xs text-[var(--text-secondary)]">{{ mount.uid_maps?.join(', ') || '—' }}</td>
            <td class="px-4 py-2.5 text-xs text-[var(--text-secondary)]">{{ mount.gid_maps?.join(', ') || '—' }}</td>
            <td class="px-4 py-2.5 text-right">
              <button
                class="p-1 rounded hover:bg-[var(--danger)] transition-colors"
                title="Remove"
                @click="confirmRemove(mount.target_path)"
              >
                <Trash2 class="w-4 h-4" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else-if="!loading" class="text-[var(--text-secondary)] text-sm">
      No mounts
    </div>

    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />
  </div>
</template>
