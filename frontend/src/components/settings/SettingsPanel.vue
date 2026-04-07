<script setup>
import { ref, onMounted } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import ActionButton from '../shared/ActionButton.vue'
import { Save } from 'lucide-vue-next'

const toasts = useToastStore()
const loading = ref(true)
const saving = ref(false)

const cpus = ref(2)
const memoryMB = ref(1024)
const diskGB = ref(8)
const sshPublicKey = ref('')
const sshPrivateKey = ref('')
const playbooksDir = ref('')
const ansibleInstalled = ref(false)
const ansibleVersion = ref('')

onMounted(async () => {
  try {
    const [defaults, status] = await Promise.all([
      api.getVMDefaults(),
      api.getAnsibleStatus(),
    ])
    cpus.value = defaults.cpus
    memoryMB.value = defaults.memory_mb
    diskGB.value = defaults.disk_gb
    sshPublicKey.value = defaults.ssh_public_key || ''
    sshPrivateKey.value = defaults.ssh_private_key || ''
    playbooksDir.value = status.playbooks_dir || ''
    ansibleInstalled.value = status.installed
    ansibleVersion.value = status.version || ''
  } catch (e) {
    toasts.error('Failed to load settings')
  } finally {
    loading.value = false
  }
})

async function save() {
  saving.value = true
  try {
    const updated = await api.updateVMDefaults({
      cpus: Number(cpus.value),
      memory_mb: Number(memoryMB.value),
      disk_gb: Number(diskGB.value),
      ssh_public_key: sshPublicKey.value.trim(),
      ssh_private_key: sshPrivateKey.value.trim(),
    })
    cpus.value = updated.cpus
    memoryMB.value = updated.memory_mb
    diskGB.value = updated.disk_gb
    sshPublicKey.value = updated.ssh_public_key || ''
    sshPrivateKey.value = updated.ssh_private_key || ''
    toasts.success('VM defaults saved')
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="p-6">
    <h2 class="text-xl font-semibold mb-6">Settings</h2>

    <div v-if="loading" class="text-sm text-[var(--text-secondary)]">Loading...</div>

    <div v-else>
      <h3 class="text-sm font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-4">Default VM Specs</h3>
      <p class="text-xs text-[var(--muted)] mb-6">These values are used as defaults when creating new VMs.</p>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-6 max-w-2xl mb-6">
        <!-- CPUs -->
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">CPUs</label>
          <input
            v-model.number="cpus"
            type="number"
            min="1"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          />
        </div>

        <!-- Memory -->
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">Memory (MB)</label>
          <input
            v-model.number="memoryMB"
            type="number"
            min="512"
            step="256"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          />
        </div>

        <!-- Disk -->
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">Disk (GB)</label>
          <input
            v-model.number="diskGB"
            type="number"
            min="1"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          />
        </div>
      </div>

      <hr class="my-6 border-[var(--border)]" />

      <h3 class="text-sm font-medium text-[var(--text-secondary)] uppercase tracking-wider mb-4">Ansible</h3>

      <div class="max-w-2xl mb-4 bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-3">
        <div class="flex items-center gap-4 text-sm">
          <div class="flex items-center gap-2">
            <span class="text-[var(--text-secondary)]">Status:</span>
            <span v-if="ansibleInstalled" class="text-[var(--success)]">Installed</span>
            <span v-else class="text-[var(--danger)]">Not found</span>
          </div>
          <span v-if="ansibleVersion" class="text-xs text-[var(--muted)]">{{ ansibleVersion }}</span>
        </div>
        <div v-if="playbooksDir" class="mt-2 text-sm">
          <span class="text-[var(--text-secondary)]">Playbooks directory:</span>
          <code class="ml-2 px-2 py-0.5 rounded bg-[var(--bg-primary)] text-[var(--text-primary)] text-xs font-mono">{{ playbooksDir }}</code>
        </div>
      </div>

      <p class="text-xs text-[var(--muted)] mb-3">SSH keys for Ansible access. The public key is copied to VMs; the private key path is included in generated inventory files.</p>
      <div class="max-w-2xl mb-6 space-y-4">
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">SSH Public Key</label>
          <textarea
            v-model="sshPublicKey"
            rows="3"
            placeholder="ssh-ed25519 AAAA... user@host"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent)] resize-none"
          />
        </div>
        <div>
          <label class="block text-sm text-[var(--text-secondary)] mb-1">SSH Private Key Path</label>
          <input
            v-model="sshPrivateKey"
            type="text"
            placeholder="~/.ssh/id_ed25519"
            class="w-full px-3 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent)]"
          />
        </div>
      </div>

      <ActionButton label="Save" :icon="Save" variant="success" :disabled="saving" @click="save" />
    </div>
  </div>
</template>
