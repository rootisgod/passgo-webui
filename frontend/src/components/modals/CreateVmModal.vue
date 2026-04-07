<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { Loader2, Save } from 'lucide-vue-next'

const emit = defineEmits(['close'])
const store = useVmStore()
const toasts = useToastStore()

const name = ref('')
const release = ref('')
const cpus = ref(2)
const memoryMB = ref(1024)
const diskGB = ref(8)
const cloudInit = ref('')
const network = ref('')
const playbook = ref('')
const submitting = ref(false)
const selectedProfile = ref('')

const images = ref([])
const loadingImages = ref(true)
const networks = ref([])
const templates = ref([])
const playbooks = ref([])

const placeholder = ref('VM-????')

// Save as profile state
const showSaveProfile = ref(false)
const profileId = ref('')
const profileName = ref('')
const savingProfile = ref(false)

const imageList = computed(() => images.value.filter(i => i.type === 'image'))
const blueprintList = computed(() => images.value.filter(i => i.type === 'blueprint'))
const userTemplates = computed(() => templates.value.filter(t => !t.builtIn))
const builtInTemplates = computed(() => templates.value.filter(t => t.builtIn))

const selectedProfileObj = computed(() =>
  store.profiles.find(p => p.id === selectedProfile.value)
)

function imageLabel(img) {
  const aliases = img.aliases?.length ? ` (${img.aliases.join(', ')})` : ''
  if (img.type === 'blueprint') return `${img.name} — ${img.release}`
  return `${img.name} — ${img.os} ${img.release}${aliases}`
}

// When profile changes, pre-fill form
watch(selectedProfile, (id) => {
  if (!id) return
  const p = store.profiles.find(pr => pr.id === id)
  if (!p) return
  if (p.release) release.value = p.release
  if (p.cpus) cpus.value = p.cpus
  if (p.memory_mb) memoryMB.value = p.memory_mb
  if (p.disk_gb) diskGB.value = p.disk_gb
  if (p.cloud_init) cloudInit.value = p.cloud_init
  if (p.network) network.value = p.network
  if (p.playbook) playbook.value = p.playbook
})

let defaultCpus = 2
let defaultMemory = 1024
let defaultDisk = 8

onMounted(async () => {
  // Generate placeholder name
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let rand = ''
  for (let i = 0; i < 4; i++) rand += chars[Math.floor(Math.random() * chars.length)]
  placeholder.value = 'VM-' + rand

  // Load images, networks, templates, playbooks, and VM defaults in parallel
  try {
    const [imgs, nets, tmpls, defaults, pbs] = await Promise.all([
      api.listImages().catch(() => []),
      api.listNetworks().catch(() => []),
      api.listCloudInitTemplates().catch(() => []),
      api.getVMDefaults().catch(() => null),
      api.listPlaybooks().catch(() => []),
    ])
    if (defaults) {
      cpus.value = defaults.cpus
      memoryMB.value = defaults.memory_mb
      diskGB.value = defaults.disk_gb
      defaultCpus = defaults.cpus
      defaultMemory = defaults.memory_mb
      defaultDisk = defaults.disk_gb
    }
    images.value = Array.isArray(imgs) ? imgs : []
    networks.value = Array.isArray(nets) ? nets : []
    templates.value = Array.isArray(tmpls) ? tmpls : []
    playbooks.value = Array.isArray(pbs) ? pbs : []
    // Default to first image (usually latest LTS)
    if (images.value.length && !release.value) {
      const lts = images.value.find(i => i.aliases?.includes('lts'))
      release.value = lts ? lts.name : images.value[0].name
    }
  } catch { /* ignore */ }
  loadingImages.value = false
})

async function submit() {
  submitting.value = true
  try {
    const opts = {
      name: name.value || '',
      release: release.value,
      cpus: Number(cpus.value),
      memoryMB: Number(memoryMB.value),
      diskGB: Number(diskGB.value),
    }
    if (cloudInit.value) {
      opts.cloudInit = cloudInit.value
    }
    if (network.value) {
      opts.network = network.value
    }
    if (playbook.value) {
      opts.playbook = playbook.value
    }
    if (selectedProfile.value) {
      opts.profile = selectedProfile.value
    }
    await api.createVM(opts)
    toasts.success(`VM creation started`)
    store.fetchVMs()
    emit('close')
  } catch (e) {
    toasts.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function saveAsProfile() {
  if (!profileId.value || !profileName.value) return
  savingProfile.value = true
  try {
    const profile = {
      id: profileId.value,
      name: profileName.value,
      release: release.value,
      cpus: Number(cpus.value) !== defaultCpus ? Number(cpus.value) : 0,
      memory_mb: Number(memoryMB.value) !== defaultMemory ? Number(memoryMB.value) : 0,
      disk_gb: Number(diskGB.value) !== defaultDisk ? Number(diskGB.value) : 0,
      cloud_init: cloudInit.value || '',
      network: network.value || '',
    }
    await api.createProfile(profile)
    toasts.success(`Profile "${profileName.value}" saved`)
    store.fetchProfiles()
    showSaveProfile.value = false
    profileId.value = ''
    profileName.value = ''
  } catch (e) {
    toasts.error(e.message)
  } finally {
    savingProfile.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-40 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="emit('close')" />
      <div class="relative bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-6 max-w-lg w-full mx-4 shadow-2xl max-h-[90vh] overflow-y-auto">
        <h3 class="text-lg font-semibold mb-6">Create Virtual Machine</h3>

        <div class="space-y-4">
          <!-- Profile -->
          <div v-if="store.profiles.length > 0">
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Profile</label>
            <select
              v-model="selectedProfile"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
              <option value="">No profile</option>
              <option v-for="p in store.profiles" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
            <p v-if="selectedProfileObj?.playbook" class="text-xs text-[var(--accent)] mt-1">
              Will auto-run: {{ selectedProfileObj.playbook }}
            </p>
            <p v-if="selectedProfileObj?.group" class="text-xs text-[var(--text-secondary)] mt-1">
              Auto-assign to group: {{ selectedProfileObj.group }}
            </p>
          </div>

          <!-- Name -->
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Name</label>
            <input
              v-model="name"
              type="text"
              :placeholder="placeholder"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
            <p class="text-xs text-[var(--text-secondary)] mt-1">Leave empty for auto-generated name</p>
          </div>

          <!-- Image -->
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Image</label>
            <select
              v-model="release"
              :disabled="loadingImages"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)] disabled:opacity-50"
            >
              <option v-if="loadingImages" value="" disabled>Loading images...</option>
              <optgroup v-if="imageList.length" label="Images">
                <option v-for="img in imageList" :key="img.name" :value="img.name">{{ imageLabel(img) }}</option>
              </optgroup>
              <optgroup v-if="blueprintList.length" label="Blueprints (Deprecating Soon...)">
                <option v-for="img in blueprintList" :key="img.name" :value="img.name">{{ imageLabel(img) }}</option>
              </optgroup>
            </select>
          </div>

          <!-- Resources -->
          <div class="grid grid-cols-3 gap-4">
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">CPUs</label>
              <input
                v-model.number="cpus"
                type="number"
                :min="1"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
              />
            </div>
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">RAM (MB)</label>
              <input
                v-model.number="memoryMB"
                type="number"
                :min="512"
                :step="256"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
              />
            </div>
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Disk (GB)</label>
              <input
                v-model.number="diskGB"
                type="number"
                :min="1"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
              />
            </div>
          </div>

          <!-- Cloud-init -->
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Cloud-Init Template</label>
            <select
              v-model="cloudInit"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
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
            <select
              v-model="network"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
              <option value="">Default (NAT)</option>
              <optgroup label="Bridged Networks">
                <option value="bridged">Auto-detect</option>
                <option v-for="n in networks" :key="n.name" :value="n.name">
                  {{ n.name }} — {{ n.type }}{{ n.description ? ' (' + n.description + ')' : '' }}
                </option>
              </optgroup>
            </select>
          </div>

          <!-- Playbook (auto-run after launch) -->
          <div v-if="playbooks.length > 0">
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Run Playbook After Launch</label>
            <select
              v-model="playbook"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
              <option value="">None</option>
              <option v-for="pb in playbooks" :key="pb.name" :value="pb.name">{{ pb.name }}</option>
            </select>
          </div>
        </div>

        <!-- Save as Profile -->
        <div v-if="showSaveProfile" class="mt-4 p-3 bg-[var(--bg-primary)] rounded border border-[var(--border)]">
          <p class="text-xs text-[var(--text-secondary)] mb-2 font-medium">Save current settings as a profile</p>
          <div class="grid grid-cols-2 gap-2 mb-2">
            <input
              v-model="profileId"
              type="text"
              placeholder="Profile ID (e.g. k8s-node)"
              class="bg-[var(--bg-surface)] border border-[var(--border)] rounded px-2 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
            <input
              v-model="profileName"
              type="text"
              placeholder="Display name"
              class="bg-[var(--bg-surface)] border border-[var(--border)] rounded px-2 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
          </div>
          <div class="flex justify-end gap-2">
            <button @click="showSaveProfile = false" class="px-3 py-1 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors">Cancel</button>
            <button
              @click="saveAsProfile"
              :disabled="!profileId || !profileName || savingProfile"
              class="px-3 py-1 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
            >Save Profile</button>
          </div>
        </div>

        <div class="flex justify-between gap-3 mt-6">
          <button
            v-if="!showSaveProfile"
            @click="showSaveProfile = true"
            class="flex items-center gap-1.5 px-3 py-2 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors text-[var(--text-secondary)]"
          >
            <Save class="w-3.5 h-3.5" />
            Save as Profile
          </button>
          <div v-else />
          <div class="flex gap-3">
            <button
              @click="emit('close')"
              class="px-4 py-2 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
            >Cancel</button>
            <button
              @click="submit"
              :disabled="submitting"
              class="flex items-center gap-2 px-4 py-2 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
            >
              <Loader2 v-if="submitting" class="w-4 h-4 animate-spin" />
              Create
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
