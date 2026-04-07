<script setup>
import { ref, computed, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { Plus, Pencil, Trash2, Save, X } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()

const loading = ref(true)
const editing = ref(null) // profile id being edited, or '__new__' for new
const saving = ref(false)

// Form state
const formId = ref('')
const formName = ref('')
const formRelease = ref('')
const formCpus = ref(0)
const formMemoryMB = ref(0)
const formDiskGB = ref(0)
const formCloudInit = ref('')
const formNetwork = ref('')
const formPlaybook = ref('')
const formGroup = ref('')

// Options for dropdowns
const images = ref([])
const networks = ref([])
const templates = ref([])
const playbooks = ref([])

const imageList = computed(() => images.value.filter(i => i.type === 'image'))
const blueprintList = computed(() => images.value.filter(i => i.type === 'blueprint'))
const userTemplates = computed(() => templates.value.filter(t => !t.builtIn))
const builtInTemplates = computed(() => templates.value.filter(t => t.builtIn))

function imageLabel(img) {
  const aliases = img.aliases?.length ? ` (${img.aliases.join(', ')})` : ''
  if (img.type === 'blueprint') return `${img.name} — ${img.release}`
  return `${img.name} — ${img.os} ${img.release}${aliases}`
}

onMounted(async () => {
  try {
    const [imgs, nets, tmpls, pbs] = await Promise.all([
      api.listImages().catch(() => []),
      api.listNetworks().catch(() => []),
      api.listCloudInitTemplates().catch(() => []),
      api.listPlaybooks().catch(() => []),
    ])
    images.value = Array.isArray(imgs) ? imgs : []
    networks.value = Array.isArray(nets) ? nets : []
    templates.value = Array.isArray(tmpls) ? tmpls : []
    playbooks.value = Array.isArray(pbs) ? pbs : []
  } catch { /* ignore */ }
  loading.value = false
})

function startNew() {
  editing.value = '__new__'
  formId.value = ''
  formName.value = ''
  formRelease.value = ''
  formCpus.value = 0
  formMemoryMB.value = 0
  formDiskGB.value = 0
  formCloudInit.value = ''
  formNetwork.value = ''
  formPlaybook.value = ''
  formGroup.value = ''
}

function startEdit(p) {
  editing.value = p.id
  formId.value = p.id
  formName.value = p.name
  formRelease.value = p.release || ''
  formCpus.value = p.cpus || 0
  formMemoryMB.value = p.memory_mb || 0
  formDiskGB.value = p.disk_gb || 0
  formCloudInit.value = p.cloud_init || ''
  formNetwork.value = p.network || ''
  formPlaybook.value = p.playbook || ''
  formGroup.value = p.group || ''
}

function cancelEdit() {
  editing.value = null
}

async function saveProfile() {
  saving.value = true
  try {
    const profile = {
      id: formId.value,
      name: formName.value,
      release: formRelease.value,
      cpus: Number(formCpus.value),
      memory_mb: Number(formMemoryMB.value),
      disk_gb: Number(formDiskGB.value),
      cloud_init: formCloudInit.value,
      network: formNetwork.value,
      playbook: formPlaybook.value,
      group: formGroup.value,
    }
    if (editing.value === '__new__') {
      await api.createProfile(profile)
      toasts.success(`Profile "${formName.value}" created`)
    } else {
      await api.updateProfile(editing.value, profile)
      toasts.success(`Profile "${formName.value}" updated`)
    }
    await store.fetchProfiles()
    editing.value = null
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}

async function deleteProfile(id, name) {
  if (!confirm(`Delete profile "${name}"?`)) return
  try {
    await api.deleteProfile(id)
    toasts.success(`Profile "${name}" deleted`)
    if (editing.value === id) editing.value = null
    await store.fetchProfiles()
  } catch (e) {
    toasts.error(e.message)
  }
}

function profileSummary(p) {
  const parts = []
  if (p.release) parts.push(p.release)
  if (p.cpus) parts.push(`${p.cpus} CPU`)
  if (p.memory_mb) parts.push(`${p.memory_mb} MB`)
  if (p.disk_gb) parts.push(`${p.disk_gb} GB`)
  if (p.playbook) parts.push(p.playbook)
  if (p.group) parts.push(`[${p.group}]`)
  return parts.join(' · ') || 'Default settings'
}
</script>

<template>
  <div class="h-full flex flex-col">
    <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
      <h2 class="text-lg font-semibold">Launch Profiles</h2>
      <button
        @click="startNew"
        :disabled="editing === '__new__'"
        class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
      >
        <Plus class="w-3.5 h-3.5" />
        New Profile
      </button>
    </div>

    <div v-if="loading" class="flex-1 flex items-center justify-center text-[var(--text-secondary)] text-sm">
      Loading...
    </div>

    <div v-else class="flex-1 overflow-y-auto">
      <!-- Empty state -->
      <div v-if="store.profiles.length === 0 && editing !== '__new__'" class="p-6 text-center text-[var(--text-secondary)] text-sm">
        <p>No launch profiles yet.</p>
        <p class="mt-1">Profiles save VM recipes (image, specs, cloud-init, playbook) for one-click launches.</p>
      </div>

      <!-- Profile list -->
      <div class="divide-y divide-[var(--border)]">
        <!-- New profile form -->
        <div v-if="editing === '__new__'" class="p-4 bg-[var(--bg-primary)]/50">
          <div class="space-y-3">
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">ID</label>
                <input v-model="formId" type="text" placeholder="e.g. k8s-node"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Display Name</label>
                <input v-model="formName" type="text" placeholder="e.g. Kubernetes Node"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>
            </div>

            <!-- Image -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Image</label>
              <select v-model="formRelease"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="">Use default</option>
                <optgroup v-if="imageList.length" label="Images">
                  <option v-for="img in imageList" :key="img.name" :value="img.name">{{ imageLabel(img) }}</option>
                </optgroup>
                <optgroup v-if="blueprintList.length" label="Blueprints">
                  <option v-for="img in blueprintList" :key="img.name" :value="img.name">{{ imageLabel(img) }}</option>
                </optgroup>
              </select>
            </div>

            <!-- Resources -->
            <div class="grid grid-cols-3 gap-3">
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">CPUs (0=default)</label>
                <input v-model.number="formCpus" type="number" :min="0"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">RAM MB (0=default)</label>
                <input v-model.number="formMemoryMB" type="number" :min="0" :step="256"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Disk GB (0=default)</label>
                <input v-model.number="formDiskGB" type="number" :min="0"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>
            </div>

            <!-- Cloud-init -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Cloud-Init Template</label>
              <select v-model="formCloudInit"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="">None</option>
                <optgroup v-if="userTemplates.length" label="User Templates">
                  <option v-for="t in userTemplates" :key="t.path" :value="t.path">{{ t.label }}</option>
                </optgroup>
                <optgroup v-if="builtInTemplates.length" label="Built-in Templates">
                  <option v-for="t in builtInTemplates" :key="t.path" :value="t.path">{{ t.label }}</option>
                </optgroup>
              </select>
            </div>

            <!-- Network -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Network</label>
              <select v-model="formNetwork"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="">Default (NAT)</option>
                <optgroup label="Bridged Networks">
                  <option value="bridged">Auto-detect</option>
                  <option v-for="n in networks" :key="n.name" :value="n.name">{{ n.name }}</option>
                </optgroup>
              </select>
            </div>

            <!-- Playbook (auto-run after launch) -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Auto-Run Playbook</label>
              <select v-model="formPlaybook"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="">None</option>
                <option v-for="pb in playbooks" :key="pb.name" :value="pb.name">{{ pb.name }}</option>
              </select>
            </div>

            <!-- Group -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Auto-Assign Group</label>
              <select v-model="formGroup"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                <option value="">None</option>
                <option v-for="g in store.groups" :key="g" :value="g">{{ g }}</option>
              </select>
            </div>

            <div class="flex justify-end gap-2 pt-1">
              <button @click="cancelEdit"
                class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors">
                <X class="w-3.5 h-3.5" /> Cancel
              </button>
              <button @click="saveProfile" :disabled="!formId || !formName || saving"
                class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40">
                <Save class="w-3.5 h-3.5" /> {{ editing === '__new__' ? 'Create' : 'Save' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Existing profiles -->
        <div v-for="p in store.profiles" :key="p.id">
          <!-- Edit mode -->
          <div v-if="editing === p.id" class="p-4 bg-[var(--bg-primary)]/50">
            <div class="space-y-3">
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Display Name</label>
                <input v-model="formName" type="text"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Image</label>
                <select v-model="formRelease"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="">Use default</option>
                  <optgroup v-if="imageList.length" label="Images">
                    <option v-for="img in imageList" :key="img.name" :value="img.name">{{ imageLabel(img) }}</option>
                  </optgroup>
                  <optgroup v-if="blueprintList.length" label="Blueprints">
                    <option v-for="img in blueprintList" :key="img.name" :value="img.name">{{ imageLabel(img) }}</option>
                  </optgroup>
                </select>
              </div>

              <div class="grid grid-cols-3 gap-3">
                <div>
                  <label class="block text-xs text-[var(--text-secondary)] mb-1">CPUs (0=default)</label>
                  <input v-model.number="formCpus" type="number" :min="0"
                    class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
                </div>
                <div>
                  <label class="block text-xs text-[var(--text-secondary)] mb-1">RAM MB (0=default)</label>
                  <input v-model.number="formMemoryMB" type="number" :min="0" :step="256"
                    class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
                </div>
                <div>
                  <label class="block text-xs text-[var(--text-secondary)] mb-1">Disk GB (0=default)</label>
                  <input v-model.number="formDiskGB" type="number" :min="0"
                    class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
                </div>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Cloud-Init Template</label>
                <select v-model="formCloudInit"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="">None</option>
                  <optgroup v-if="userTemplates.length" label="User Templates">
                    <option v-for="t in userTemplates" :key="t.path" :value="t.path">{{ t.label }}</option>
                  </optgroup>
                  <optgroup v-if="builtInTemplates.length" label="Built-in Templates">
                    <option v-for="t in builtInTemplates" :key="t.path" :value="t.path">{{ t.label }}</option>
                  </optgroup>
                </select>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Network</label>
                <select v-model="formNetwork"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="">Default (NAT)</option>
                  <optgroup label="Bridged Networks">
                    <option value="bridged">Auto-detect</option>
                    <option v-for="n in networks" :key="n.name" :value="n.name">{{ n.name }}</option>
                  </optgroup>
                </select>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Auto-Run Playbook</label>
                <select v-model="formPlaybook"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="">None</option>
                  <option v-for="pb in playbooks" :key="pb.name" :value="pb.name">{{ pb.name }}</option>
                </select>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Auto-Assign Group</label>
                <select v-model="formGroup"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]">
                  <option value="">None</option>
                  <option v-for="g in store.groups" :key="g" :value="g">{{ g }}</option>
                </select>
              </div>

              <div class="flex justify-end gap-2 pt-1">
                <button @click="cancelEdit"
                  class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors">
                  <X class="w-3.5 h-3.5" /> Cancel
                </button>
                <button @click="saveProfile" :disabled="!formName || saving"
                  class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40">
                  <Save class="w-3.5 h-3.5" /> Save
                </button>
              </div>
            </div>
          </div>

          <!-- View mode -->
          <div v-else class="flex items-center gap-3 px-4 py-3 hover:bg-[var(--bg-hover)] transition-colors">
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium text-[var(--text-primary)]">{{ p.name }}</div>
              <div class="text-xs text-[var(--text-secondary)] truncate">{{ profileSummary(p) }}</div>
            </div>
            <button @click="startEdit(p)"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)]">
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button @click="deleteProfile(p.id, p.name)"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--danger)]">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
