<script setup>
import { computed, ref, watch } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import { usePolling } from '../../composables/usePolling.js'
import * as api from '../../api/client.js'
import ConfirmModal from '../modals/ConfirmModal.vue'
import { Clock, Copy, ExternalLink, KeyRound, Network, Plus, RefreshCw, Save, Shield, Terminal, Trash2, X } from 'lucide-vue-next'

const props = defineProps({
  vmName: { type: String, default: '' },
  embedded: { type: Boolean, default: false },
})

const store = useVmStore()
const toasts = useToastStore()

const rules = ref([])
const loading = ref(false)
const showForm = ref(false)
const saving = ref(false)
const confirmAction = ref(null)
const oneTimeAccess = ref({})

const formVm = ref('')
const formProtocol = ref('http')
const formPort = ref(5173)
const formHostPort = ref(2222)
const formAutoHostPort = ref(true)
const formBindAddress = ref('0.0.0.0')
const formLabel = ref('')
const formOwner = ref('')
const formTTL = ref('none')
const formEnabled = ref(true)

const title = computed(() => props.vmName ? 'Proxy Ports' : 'Proxies')
const selectedIsTemplate = computed(() => props.vmName ? store.isTemplate(props.vmName) : false)
const selectableVms = computed(() => store.vms.filter(vm => vm.state !== 'Deleted' && !store.isTemplate(vm.name)))
const hasRules = computed(() => rules.value.length > 0)
const isSSHForm = computed(() => formProtocol.value === 'ssh')
const targetPortLabel = computed(() => isSSHForm.value ? 'VM SSH Port' : 'HTTP/WS Port')
const ttlOptions = [
  { value: 'none', label: 'No expiry' },
  { value: '60', label: '1 hour' },
  { value: '240', label: '4 hours' },
  { value: '1440', label: '1 day' },
]
const bindOptions = [
  { value: '0.0.0.0', label: 'LAN / all' },
  { value: '127.0.0.1', label: 'Local only' },
]
const canCreate = computed(() => {
  const vm = props.vmName || formVm.value
  const targetPort = Number(formPort.value)
  const hostPort = Number(formHostPort.value)
  return !!vm &&
    !selectedIsTemplate.value &&
    targetPort >= 1 &&
    targetPort <= 65535 &&
    (!isSSHForm.value || formAutoHostPort.value || (hostPort >= 1 && hostPort <= 65535))
})

async function loadRules() {
  loading.value = true
  try {
    const data = await api.listProxyRules(props.vmName || '')
    rules.value = Array.isArray(data) ? data : []
  } catch (e) {
    toasts.error(e.message)
  } finally {
    loading.value = false
  }
}

function startForm() {
  showForm.value = true
  formVm.value = props.vmName || ''
  formProtocol.value = 'http'
  formPort.value = 5173
  formHostPort.value = 2222
  formAutoHostPort.value = true
  formBindAddress.value = '0.0.0.0'
  formLabel.value = ''
  formOwner.value = ''
  formTTL.value = 'none'
  formEnabled.value = true
}

function cancelForm() {
  showForm.value = false
}

async function createRule() {
  if (!canCreate.value) return
  saving.value = true
  try {
    const vm = props.vmName || formVm.value
    const payload = {
      vm,
      protocol: formProtocol.value,
      port: Number(formPort.value),
      label: formLabel.value.trim(),
      owner: formOwner.value.trim(),
      enabled: formEnabled.value,
    }
    if (formTTL.value !== 'none') {
      payload.ttl_minutes = Number(formTTL.value)
    }
    if (isSSHForm.value) {
      payload.auto_host_port = formAutoHostPort.value
      if (!formAutoHostPort.value) {
        payload.host_port = Number(formHostPort.value)
      }
      payload.bind_address = formBindAddress.value
    } else {
      payload.generate_token = true
    }
    const created = await api.createProxyRule(payload)
    if (created?.id && created?.proxy_url) {
      oneTimeAccess.value = { ...oneTimeAccess.value, [created.id]: created.proxy_url }
    }
    const access = isSSHForm.value ? `host port ${created?.host_port || Number(formHostPort.value)}` : `${vm}:${Number(formPort.value)}`
    toasts.success(`Proxy rule added for ${access}`)
    showForm.value = false
    await loadRules()
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}

async function cleanupExpired() {
  try {
    const result = await api.cleanupProxyRules()
    toasts.success(`Removed ${result.removed || 0} expired rules`)
    await loadRules()
  } catch (e) {
    toasts.error(e.message)
  }
}

async function toggleRule(rule) {
  try {
    await api.updateProxyRule(rule.id, { enabled: !rule.enabled })
    await loadRules()
  } catch (e) {
    toasts.error(e.message)
  }
}

function confirmDelete(rule) {
  confirmAction.value = {
    message: `Delete proxy rule for ${rule.vm}:${rule.port}?`,
    fn: async () => {
      try {
        await api.deleteProxyRule(rule.id)
        toasts.success('Proxy rule deleted')
        await loadRules()
      } catch (e) {
        toasts.error(e.message)
      }
    },
  }
}

async function executeConfirmed() {
  const fn = confirmAction.value?.fn
  confirmAction.value = null
  if (fn) await fn()
}

async function copyURL(rule) {
  const value = accessValue(rule)
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    toasts.success(isSSHRule(rule) ? 'SSH command copied' : 'Proxy URL copied')
  } catch {
    toasts.error(isSSHRule(rule) ? 'Failed to copy SSH command' : 'Failed to copy URL')
  }
}

function openURL(rule) {
  const value = accessValue(rule)
  if (isSSHRule(rule) || !value) return
  window.open(value, '_blank', 'noopener,noreferrer')
}

function statusClass(status) {
  switch (status) {
    case 'live': return 'border-green-800 bg-green-900/25 text-[var(--success)]'
    case 'disabled': return 'border-gray-700 bg-gray-800/40 text-[var(--muted)]'
    case 'expired': return 'border-yellow-800 bg-yellow-900/25 text-[var(--warning)]'
    case 'vm_stopped': return 'border-yellow-800 bg-yellow-900/25 text-[var(--warning)]'
    case 'vm_missing': return 'border-red-800 bg-red-900/25 text-[var(--danger)]'
    case 'no_ip': return 'border-red-800 bg-red-900/25 text-[var(--danger)]'
    case 'listen_error': return 'border-red-800 bg-red-900/25 text-[var(--danger)]'
    default: return 'border-gray-700 bg-gray-800/30 text-[var(--text-secondary)]'
  }
}

function statusLabel(status) {
  return String(status || 'unknown').replaceAll('_', ' ')
}

function shortURL(url) {
  if (!url) return ''
  try {
    const parsed = new URL(url)
    return parsed.host + parsed.pathname
  } catch {
    return url
  }
}

function protocolLabel(rule) {
  return rule.protocol || 'HTTP/WS'
}

function isSSHRule(rule) {
  return String(rule.protocol || '').toLowerCase() === 'ssh' || !!rule.ssh_command
}

function accessValue(rule) {
  return oneTimeAccess.value[rule.id] || (isSSHRule(rule) ? rule.ssh_command : rule.proxy_url)
}

function accessDisplay(rule) {
  if (oneTimeAccess.value[rule.id]) return shortURL(oneTimeAccess.value[rule.id])
  if (isSSHRule(rule)) return rule.ssh_command || ''
  return shortURL(rule.proxy_url)
}

function destinationLabel(rule) {
  return rule.destination || `${rule.vm}:${rule.port}`
}

function listenLabel(rule) {
  if (!isSSHRule(rule)) return ''
  return rule.listen_address || (rule.host_port ? `:${rule.host_port}` : '')
}

function exposureLabel(rule) {
  if (isSSHRule(rule)) {
    return rule.bind_address === '127.0.0.1' ? 'local' : 'lan'
  }
  return rule.token_required ? `token ${rule.token_prefix || ''}`.trim() : 'session'
}

function accessMeta(rule) {
  const pieces = []
  if (rule.owner) pieces.push(rule.owner)
  if (rule.access_count) pieces.push(`${rule.access_count} hits`)
  if (rule.last_accessed_at) pieces.push(`last ${formatExpiry(rule.last_accessed_at)}`)
  return pieces.join(' · ')
}

function formatExpiry(value) {
  if (!value) return 'None'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

watch(() => props.vmName, () => {
  showForm.value = false
  loadRules()
})

watch(formProtocol, (protocol) => {
  if (protocol === 'ssh' && Number(formPort.value) === 5173) {
    formPort.value = 22
    formTTL.value = '240'
  }
  if (protocol === 'http' && Number(formPort.value) === 22) {
    formPort.value = 5173
    if (formTTL.value === '240') formTTL.value = 'none'
  }
})

usePolling(loadRules, 5000)
</script>

<template>
  <div class="h-full flex flex-col" :class="embedded ? 'p-6' : ''">
    <div
      class="flex items-center justify-between"
      :class="embedded ? 'mb-4' : 'px-6 py-4 border-b border-[var(--border)]'"
    >
      <div class="flex items-center gap-2 min-w-0">
        <Network class="w-4 h-4 text-[var(--accent)] flex-shrink-0" />
        <h2 class="text-lg font-semibold truncate">{{ title }}</h2>
      </div>
      <div class="flex items-center gap-2">
        <button
          class="p-2 rounded hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
          title="Refresh"
          @click="loadRules"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </button>
        <button
          class="p-2 rounded hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
          title="Cleanup expired"
          @click="cleanupExpired"
        >
          <Clock class="w-4 h-4" />
        </button>
        <button
          class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
          :disabled="showForm || selectedIsTemplate"
          @click="startForm"
        >
          <Plus class="w-3.5 h-3.5" />
          Add Rule
        </button>
      </div>
    </div>

    <div class="flex-1 overflow-auto" :class="embedded ? '' : 'p-6'">
      <div v-if="selectedIsTemplate" class="text-sm text-[var(--text-secondary)] mb-4">
        Remove the template tag to add proxy rules.
      </div>
      <div v-if="showForm" class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] p-4 mb-4">
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-8 gap-3">
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Protocol</label>
            <div class="inline-flex w-full rounded border border-[var(--border)] overflow-hidden bg-[var(--bg-primary)]">
              <button
                type="button"
                class="flex-1 px-3 py-1.5 text-xs transition-colors"
                :class="formProtocol === 'http' ? 'bg-[var(--accent)] text-white' : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                @click="formProtocol = 'http'"
              >
                HTTP/WS
              </button>
              <button
                type="button"
                class="flex-1 inline-flex items-center justify-center gap-1 px-3 py-1.5 text-xs transition-colors border-l border-[var(--border)]"
                :class="formProtocol === 'ssh' ? 'bg-[var(--accent)] text-white' : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                @click="formProtocol = 'ssh'"
              >
                <Terminal class="w-3.5 h-3.5" />
                SSH
              </button>
            </div>
          </div>
          <div v-if="!props.vmName">
            <label class="block text-xs text-[var(--text-secondary)] mb-1">VM</label>
            <select
              v-model="formVm"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
              <option value="" disabled>Select VM</option>
              <option v-for="vm in selectableVms" :key="vm.name" :value="vm.name">{{ vm.name }}</option>
            </select>
          </div>
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">{{ targetPortLabel }}</label>
            <input
              v-model.number="formPort"
              type="number"
              min="1"
              max="65535"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
          </div>
          <div v-if="isSSHForm && !formAutoHostPort">
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Listen Port</label>
            <input
              v-model.number="formHostPort"
              type="number"
              min="1"
              max="65535"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
          </div>
          <label v-if="isSSHForm" class="flex items-end gap-2 text-sm text-[var(--text-primary)] pb-1.5 cursor-pointer">
            <input
              v-model="formAutoHostPort"
              type="checkbox"
              class="w-4 h-4 rounded border-[var(--border)] bg-[var(--bg-primary)] text-[var(--accent)] focus:ring-0 focus:ring-offset-0"
            />
            Auto port
          </label>
          <div v-if="isSSHForm">
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Bind</label>
            <select
              v-model="formBindAddress"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
              <option v-for="option in bindOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </div>
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">TTL</label>
            <select
              v-model="formTTL"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            >
              <option v-for="option in ttlOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </div>
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Owner</label>
            <input
              v-model="formOwner"
              type="text"
              maxlength="80"
              placeholder="agent or user"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
          </div>
          <div :class="props.vmName && !isSSHForm ? 'xl:col-span-2' : ''">
            <label class="block text-xs text-[var(--text-secondary)] mb-1">Label</label>
            <input
              v-model="formLabel"
              type="text"
              maxlength="80"
              :placeholder="isSSHForm ? 'e.g. Agent SSH' : 'e.g. Vite preview'"
              class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            />
          </div>
          <label class="flex items-end gap-2 text-sm text-[var(--text-primary)] pb-1.5 cursor-pointer">
            <input
              v-model="formEnabled"
              type="checkbox"
              class="w-4 h-4 rounded border-[var(--border)] bg-[var(--bg-primary)] text-[var(--accent)] focus:ring-0 focus:ring-offset-0"
            />
            Enabled
          </label>
        </div>
        <div class="flex justify-end gap-2 mt-3">
          <button
            class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
            @click="cancelForm"
          >
            <X class="w-3.5 h-3.5" />
            Cancel
          </button>
          <button
            class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
            :disabled="!canCreate || saving"
            @click="createRule"
          >
            <Save class="w-3.5 h-3.5" />
            Create
          </button>
        </div>
      </div>

      <div v-if="hasRules" class="bg-[var(--bg-surface)] rounded-lg border border-[var(--border)] overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-[var(--border)] text-[var(--text-secondary)]">
              <th class="text-left px-4 py-2.5 font-medium">Rule</th>
              <th class="text-left px-4 py-2.5 font-medium">Protocol</th>
              <th class="text-left px-4 py-2.5 font-medium">Access</th>
              <th class="text-left px-4 py-2.5 font-medium">Destination</th>
              <th class="text-left px-4 py-2.5 font-medium">Status</th>
              <th class="text-left px-4 py-2.5 font-medium">Expires</th>
              <th class="text-right px-4 py-2.5 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="rule in rules"
              :key="rule.id"
              class="border-b border-[var(--border)] last:border-b-0 hover:bg-[var(--bg-hover)]"
            >
              <td class="px-4 py-2.5">
                <div class="font-medium text-[var(--text-primary)]">{{ rule.label || `${rule.vm}:${rule.port}` }}</div>
                <div class="text-xs text-[var(--text-secondary)] font-mono">{{ rule.vm }}:{{ rule.port }}</div>
                <div v-if="accessMeta(rule)" class="text-[10px] text-[var(--muted)] mt-0.5">{{ accessMeta(rule) }}</div>
              </td>
              <td class="px-4 py-2.5">
                <div class="flex flex-col items-start gap-1">
                  <span class="inline-flex items-center px-2 py-0.5 rounded text-xs border border-blue-800 bg-blue-900/20 text-blue-300">
                    {{ protocolLabel(rule) }}
                  </span>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] border border-gray-700 bg-gray-800/30 text-[var(--text-secondary)]">
                    <Shield class="w-3 h-3" />
                    {{ exposureLabel(rule) }}
                  </span>
                </div>
              </td>
              <td class="px-4 py-2.5">
                <button
                  v-if="accessValue(rule)"
                  class="font-mono text-xs text-[var(--accent)] hover:underline text-left break-all"
                  @click="copyURL(rule)"
                >
                  {{ accessDisplay(rule) }}
                </button>
                <span v-else class="text-xs text-[var(--muted)]">not available</span>
                <div v-if="rule.token_required" class="mt-1 inline-flex items-center gap-1 text-[10px] text-[var(--muted)]">
                  <KeyRound class="w-3 h-3" />
                  {{ rule.token_prefix || 'token protected' }}
                </div>
              </td>
              <td class="px-4 py-2.5">
                <div class="font-mono text-xs text-[var(--text-primary)]">{{ destinationLabel(rule) }}</div>
                <div v-if="listenLabel(rule)" class="font-mono text-[10px] text-[var(--text-secondary)]">listen {{ listenLabel(rule) }}</div>
                <div class="font-mono text-[10px] text-[var(--muted)]">{{ rule.target || 'internal IP pending' }}</div>
              </td>
              <td class="px-4 py-2.5">
                <span class="inline-flex items-center px-2 py-0.5 rounded text-xs border capitalize" :class="statusClass(rule.status)">
                  {{ statusLabel(rule.status) }}
                </span>
                <div v-if="rule.status_detail" class="text-[10px] text-[var(--muted)] mt-1">{{ rule.status_detail }}</div>
              </td>
              <td class="px-4 py-2.5 text-xs text-[var(--text-secondary)]">
                {{ formatExpiry(rule.expires_at) }}
              </td>
              <td class="px-4 py-2.5 text-right">
                <div class="inline-flex items-center gap-1">
                  <button
                    class="p-1 rounded hover:bg-[var(--bg-hover)] transition-colors"
                    :title="rule.enabled ? 'Disable rule' : 'Enable rule'"
                    @click="toggleRule(rule)"
                  >
                    <X v-if="rule.enabled" class="w-4 h-4" />
                    <Plus v-else class="w-4 h-4" />
                  </button>
                  <button
                    class="p-1 rounded hover:bg-[var(--bg-hover)] transition-colors"
                    :title="isSSHRule(rule) ? 'Copy SSH command' : 'Copy URL'"
                    @click="copyURL(rule)"
                  >
                    <Copy class="w-4 h-4" />
                  </button>
                  <button
                    class="p-1 rounded hover:bg-[var(--bg-hover)] transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    title="Open"
                    :disabled="isSSHRule(rule) || rule.status !== 'live'"
                    @click="openURL(rule)"
                  >
                    <ExternalLink class="w-4 h-4" />
                  </button>
                  <button
                    class="p-1 rounded hover:bg-[var(--danger)] transition-colors"
                    title="Delete"
                    @click="confirmDelete(rule)"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else-if="!loading" class="text-sm text-[var(--text-secondary)]">
        No proxy rules configured.
      </div>
    </div>

    <ConfirmModal
      v-if="confirmAction"
      :message="confirmAction.message"
      @confirm="executeConfirmed"
      @cancel="confirmAction = null"
    />
  </div>
</template>
