<script setup>
import { ref, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { Loader2 } from 'lucide-vue-next'

const emit = defineEmits(['close'])
const store = useVmStore()
const toasts = useToastStore()

const name = ref('')
const release = ref('24.04')
const cpus = ref(2)
const memoryMB = ref(1024)
const diskGB = ref(8)
const cloudInit = ref('')
const network = ref('')
const submitting = ref(false)

const releases = ['24.04', '22.04', '20.04', '18.04', 'daily']
const networks = ref([])
const templates = ref([])

const placeholder = ref('VM-????')

onMounted(async () => {
  // Generate placeholder name
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let rand = ''
  for (let i = 0; i < 4; i++) rand += chars[Math.floor(Math.random() * chars.length)]
  placeholder.value = 'VM-' + rand

  // Load networks and templates in parallel
  try {
    const [nets, tmpls] = await Promise.all([
      api.listNetworks().catch(() => []),
      api.listCloudInitTemplates().catch(() => []),
    ])
    networks.value = Array.isArray(nets) ? nets : []
    templates.value = Array.isArray(tmpls) ? tmpls : []
  } catch { /* ignore */ }
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
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-40 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="emit('close')" />
      <div class="relative bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-6 max-w-lg w-full mx-4 shadow-2xl max-h-[90vh] overflow-y-auto">
        <h3 class="text-lg font-semibold mb-6">Create Virtual Machine</h3>

        <div class="space-y-4">
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

          <!-- Release -->
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Ubuntu Release</label>
            <select
              v-model="release"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
              <option v-for="r in releases" :key="r" :value="r">{{ r }}</option>
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
              <option v-for="t in templates" :key="t.path" :value="t.path">{{ t.label }}</option>
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
              <option value="bridged">Bridged</option>
              <option v-for="n in networks" :key="n.name" :value="n.name">
                {{ n.name }} — {{ n.type }}{{ n.description ? ' (' + n.description + ')' : '' }}
              </option>
            </select>
          </div>
        </div>

        <div class="flex justify-end gap-3 mt-6">
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
  </Teleport>
</template>
