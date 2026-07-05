<script setup>
import { ref, computed, onMounted } from 'vue'
import { X, ChevronDown, Loader2, Check, AlertCircle, RefreshCw } from 'lucide-vue-next'
import { useChatStore } from '../../stores/chatStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import { getNativeAuthStatus, listChatModels } from '../../api/client.js'

const emit = defineEmits(['close'])
const chatStore = useChatStore()
const toasts = useToastStore()

const VERCEL_GATEWAY_BASE_URL = 'https://ai-gateway.vercel.sh/v1'
const VERCEL_GATEWAY_DEFAULT_MODEL = 'anthropic/claude-sonnet-5'
const OPENROUTER_BASE_URL = 'https://openrouter.ai/api/v1'
const OPENROUTER_DEFAULT_MODEL = 'anthropic/claude-sonnet-4'
const OLLAMA_BASE_URL = 'http://localhost:11434/v1'
const OLLAMA_DEFAULT_MODEL = 'llama3.2'
const PROVIDER_GATEWAY = 'vercel_ai_gateway'
const PROVIDER_OPENAI_COMPATIBLE = 'openai_compatible'
const PROVIDER_LOCAL = 'local'
const PROVIDER_CODEX = 'codex_cli'
const PROVIDER_CLAUDE = 'claude_code_cli'

const baseUrl = ref('')
const apiKey = ref('')
const model = ref('')
const provider = ref(PROVIDER_GATEWAY)
const readOnly = ref(false)
const saving = ref(false)

// Model selector state
const models = ref([])
const modelsLoading = ref(false)
const modelsError = ref(null)
const modelSearch = ref('')
const showModelDropdown = ref(false)

// Connection test state
const connectionTested = ref(false) // true after a successful connect
const testing = ref(false)
const nativeAuthProviders = ref([])
const nativeAuthLoading = ref(false)
const nativeAuthError = ref(null)

function normalizeBaseUrl(value) {
  return (value || '').trim().replace(/\/+$/, '')
}

const filteredModels = computed(() => {
  const q = modelSearch.value.toLowerCase()
  if (!q) return models.value
  return models.value.filter(m =>
    m.id.toLowerCase().includes(q) || m.name.toLowerCase().includes(q)
  )
})

const isLocal = computed(() => {
  const u = baseUrl.value.toLowerCase()
  return u.includes('localhost') || u.includes('127.0.0.1')
})

const isGateway = computed(() => {
  return provider.value === PROVIDER_GATEWAY || normalizeBaseUrl(baseUrl.value) === VERCEL_GATEWAY_BASE_URL
})

const isNativeProvider = computed(() => provider.value === PROVIDER_CODEX || provider.value === PROVIDER_CLAUDE)

const savedBaseUrlMatches = computed(() => {
  return normalizeBaseUrl(baseUrl.value) === normalizeBaseUrl(chatStore.config.baseUrl)
})

const apiKeyBadge = computed(() => {
  if (apiKey.value) return ''
  if (!savedBaseUrlMatches.value) return ''
  if (chatStore.config.apiKeySource === 'environment' && isGateway.value) return '(server env)'
  if (chatStore.config.apiKeySource === 'stored' || chatStore.config.hasApiKey) return '(configured)'
  return ''
})

const apiKeyPlaceholder = computed(() => {
  if (apiKeyBadge.value === '(server env)') return 'Provided by server env'
  return savedBaseUrlMatches.value && chatStore.config.hasApiKey ? '••••••••' : 'Not set'
})

const apiKeyHint = computed(() => {
  if (isLocal.value) return 'Not required for local providers'
  if (isGateway.value && savedBaseUrlMatches.value && chatStore.config.apiKeySource === 'environment' && !apiKey.value) {
    return 'Using server AI_GATEWAY_API_KEY; leave blank to keep it server-side'
  }
  if (isGateway.value && !(savedBaseUrlMatches.value && chatStore.config.hasApiKey) && !apiKey.value) {
    return 'Models can be browsed without a key; chat needs a saved key or server AI_GATEWAY_API_KEY'
  }
  return 'Enter key and click Connect to browse models'
})

// Whether we can fetch models (have credentials or it's local)
const canConnect = computed(() => {
  if (isNativeProvider.value) return true
  if (!baseUrl.value) return false
  if (isGateway.value) return true
  if (isLocal.value) return true
  return !!(apiKey.value || (savedBaseUrlMatches.value && chatStore.config.hasApiKey))
})

onMounted(() => {
  baseUrl.value = chatStore.config.baseUrl
  model.value = chatStore.config.model
  provider.value = chatStore.config.provider || PROVIDER_GATEWAY
  readOnly.value = chatStore.config.readOnly
  modelSearch.value = model.value
  loadNativeAuthStatus()

  // Auto-fetch only if already configured with working credentials
  if (chatStore.config.hasApiKey || isLocal.value || isGateway.value) {
    connectAndFetchModels()
  }
})

async function loadNativeAuthStatus() {
  nativeAuthLoading.value = true
  nativeAuthError.value = null
  try {
    const result = await getNativeAuthStatus()
    nativeAuthProviders.value = result.providers || []
  } catch (e) {
    nativeAuthError.value = e.message || 'Failed to check native auth'
    nativeAuthProviders.value = []
  } finally {
    nativeAuthLoading.value = false
  }
}

function nativeAuthLabel(provider) {
  if (provider.status === 'authenticated') return 'Detected'
  if (provider.status === 'not_installed') return 'Not installed'
  if (provider.status === 'timeout') return 'Timed out'
  return 'Not detected'
}

function nativeAuthClass(provider) {
  if (provider.status === 'authenticated') return 'text-sky-300'
  if (provider.status === 'not_installed') return 'text-[var(--text-secondary)]'
  return 'text-amber-400'
}

function nativeProviderValue(nativeAuthProvider) {
  if (nativeAuthProvider.id === 'codex') return PROVIDER_CODEX
  if (nativeAuthProvider.id === 'claude') return PROVIDER_CLAUDE
  return ''
}

function nativeProviderSelected(nativeAuthProvider) {
  return provider.value === nativeProviderValue(nativeAuthProvider)
}

async function connectAndFetchModels() {
  if (!baseUrl.value && !isNativeProvider.value) return
  testing.value = true
  modelsLoading.value = true
  modelsError.value = null
  connectionTested.value = false

  try {
    // Save URL + key so the backend proxy can use them
    const saved = await chatStore.saveConfig({
      baseUrl: baseUrl.value,
      apiKey: apiKey.value || '',
      model: model.value || chatStore.config.model || '',
      provider: provider.value,
      readOnly: readOnly.value,
    })
    if (!saved) {
      throw new Error('Failed to save provider settings')
    }

    const result = await listChatModels()
    models.value = result
    connectionTested.value = true
  } catch (e) {
    modelsError.value = e.message || 'Failed to connect'
    models.value = []
  } finally {
    testing.value = false
    modelsLoading.value = false
  }
}

function selectModel(m) {
  model.value = m.id
  modelSearch.value = m.id
  showModelDropdown.value = false
}

function onModelInputFocus() {
  if (models.value.length > 0) {
    showModelDropdown.value = true
    modelSearch.value = ''
  }
}

function onModelInputBlur() {
  setTimeout(() => {
    showModelDropdown.value = false
    if (!model.value) {
      modelSearch.value = ''
    } else {
      modelSearch.value = model.value
    }
  }, 200)
}

function onModelInputChange() {
  model.value = modelSearch.value
  if (models.value.length > 0) {
    showModelDropdown.value = true
  }
}

function setPreset(preset) {
  // Reset connection state when switching provider
  connectionTested.value = false
  models.value = []
  modelsError.value = null

  if (preset === 'gateway') {
    provider.value = PROVIDER_GATEWAY
    baseUrl.value = VERCEL_GATEWAY_BASE_URL
    model.value = VERCEL_GATEWAY_DEFAULT_MODEL
    modelSearch.value = model.value
    connectAndFetchModels()
  } else if (preset === 'openrouter') {
    provider.value = PROVIDER_OPENAI_COMPATIBLE
    baseUrl.value = OPENROUTER_BASE_URL
    model.value = OPENROUTER_DEFAULT_MODEL
    modelSearch.value = model.value
  } else if (preset === 'ollama') {
    provider.value = PROVIDER_LOCAL
    baseUrl.value = OLLAMA_BASE_URL
    model.value = OLLAMA_DEFAULT_MODEL
    modelSearch.value = model.value
    // Ollama is local — connect automatically
    connectAndFetchModels()
  }
}

function setNativeProvider(nativeProvider) {
  provider.value = nativeProvider
  baseUrl.value = ''
  apiKey.value = ''
  if (nativeProvider === PROVIDER_CODEX) {
    model.value = 'codex-default'
  } else {
    model.value = 'claude-default'
  }
  modelSearch.value = model.value
  connectAndFetchModels()
}

async function save() {
  saving.value = true
  const ok = await chatStore.saveConfig({
    baseUrl: baseUrl.value,
    apiKey: apiKey.value,
    model: model.value,
    provider: provider.value,
    readOnly: readOnly.value,
  })
  saving.value = false
  if (ok) {
    toasts.success('Chat settings saved')
    emit('close')
  } else {
    toasts.error('Failed to save settings')
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" @mousedown.self="emit('close')">
      <div class="bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg shadow-xl w-[420px] max-w-[90vw]">
        <!-- Header -->
        <div class="flex items-center justify-between px-4 py-3 border-b border-[var(--border)]">
          <h3 class="font-semibold">Chat Settings</h3>
          <button @click="emit('close')" class="p-1 rounded hover:bg-[var(--bg-hover)]">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Form -->
        <div class="px-4 py-4 flex flex-col gap-4">
          <!-- Provider presets -->
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1.5">Quick Setup</label>
            <div class="flex flex-wrap gap-2">
              <button
                @click="setPreset('gateway')"
                class="px-3 py-1.5 text-xs rounded border border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors"
              >
                Vercel AI Gateway
              </button>
              <button
                @click="setPreset('openrouter')"
                class="px-3 py-1.5 text-xs rounded border border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors"
              >
                OpenRouter
              </button>
              <button
                @click="setPreset('ollama')"
                class="px-3 py-1.5 text-xs rounded border border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors"
              >
                Ollama (local)
              </button>
            </div>
          </div>

          <!-- Native CLI auth status -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="block text-xs text-[var(--text-secondary)]">Detected CLI Logins</label>
              <button
                @click="loadNativeAuthStatus"
                :disabled="nativeAuthLoading"
                class="p-1 rounded hover:bg-[var(--bg-hover)] disabled:opacity-50"
                title="Refresh native CLI auth status"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': nativeAuthLoading }" />
              </button>
            </div>
            <div class="rounded border border-[var(--border)] bg-[var(--bg-primary)] divide-y divide-[var(--border)]">
              <div
                v-for="provider in nativeAuthProviders"
                :key="provider.id"
                class="flex items-center justify-between gap-3 px-3 py-2 text-xs"
              >
                <div class="min-w-0">
                  <div class="text-[var(--text-primary)]">{{ provider.label }}</div>
                  <div class="text-[var(--text-secondary)] font-mono truncate">{{ provider.command }}</div>
                </div>
                <div class="flex items-center gap-2 shrink-0">
                  <div class="flex items-center gap-1.5" :class="nativeAuthClass(provider)">
                    <Check v-if="provider.status === 'authenticated'" class="w-3.5 h-3.5" />
                    <AlertCircle v-else class="w-3.5 h-3.5" />
                    <span>{{ nativeAuthLabel(provider) }}</span>
                  </div>
                  <button
                    v-if="provider.status === 'authenticated'"
                    @click="setNativeProvider(nativeProviderValue(provider))"
                    class="px-2 py-0.5 rounded border transition-colors"
                    :class="nativeProviderSelected(provider)
                      ? 'border-sky-700 bg-sky-900/30 text-sky-300'
                      : 'border-[var(--border)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'"
                  >
                    {{ nativeProviderSelected(provider) ? 'Selected' : 'Use' }}
                  </button>
                </div>
              </div>
              <div v-if="nativeAuthLoading && nativeAuthProviders.length === 0" class="px-3 py-2 text-xs text-[var(--text-secondary)]">
                Checking...
              </div>
            </div>
            <p v-if="nativeAuthError" class="text-xs text-red-400 mt-1">{{ nativeAuthError }}</p>
          </div>

          <!-- Base URL -->
          <div v-if="!isNativeProvider">
            <label class="block text-xs text-[var(--text-secondary)] mb-1.5">Base URL</label>
            <input
              v-model="baseUrl"
              type="text"
              :placeholder="VERCEL_GATEWAY_BASE_URL"
              class="w-full px-3 py-2 text-sm rounded border border-[var(--border)] bg-[var(--bg-primary)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
            />
          </div>

          <!-- API Key + Connect button -->
          <div v-if="!isNativeProvider">
            <label class="block text-xs text-[var(--text-secondary)] mb-1.5">
              API Key
              <span v-if="apiKeyBadge" class="text-green-400 ml-1">{{ apiKeyBadge }}</span>
            </label>
            <div class="flex gap-2">
              <input
                v-model="apiKey"
                type="password"
                :placeholder="apiKeyPlaceholder"
                class="flex-1 px-3 py-2 text-sm rounded border border-[var(--border)] bg-[var(--bg-primary)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
              />
              <button
                @click="connectAndFetchModels"
                :disabled="!canConnect || testing"
                class="flex items-center gap-1.5 px-3 py-2 text-xs rounded border transition-colors whitespace-nowrap"
                :class="{
                  'border-green-700 bg-green-900/30 text-green-400': connectionTested && !modelsError,
                  'border-red-700 bg-red-900/30 text-red-400': modelsError,
                  'border-[var(--border)] hover:bg-[var(--bg-hover)] text-[var(--text-primary)]': !connectionTested && !modelsError,
                }"
                :title="connectionTested ? 'Connected — click to refresh models' : 'Save key and fetch available models'"
              >
                <Loader2 v-if="testing" class="w-3.5 h-3.5 animate-spin" />
                <Check v-else-if="connectionTested && !modelsError" class="w-3.5 h-3.5" />
                <AlertCircle v-else-if="modelsError" class="w-3.5 h-3.5" />
                <span>{{ testing ? 'Testing...' : connectionTested && !modelsError ? 'Connected' : 'Connect' }}</span>
              </button>
            </div>
            <p class="text-xs mt-1" :class="modelsError ? 'text-red-400' : 'text-[var(--text-secondary)]'">
              {{ modelsError || apiKeyHint }}
            </p>
          </div>

          <!-- Model selector -->
          <div class="relative">
            <label class="block text-xs text-[var(--text-secondary)] mb-1.5">
              Model
              <span v-if="models.length > 0" class="text-[var(--text-secondary)]">({{ models.length }} available)</span>
            </label>
            <div class="relative">
              <input
                v-model="modelSearch"
                @focus="onModelInputFocus"
                @blur="onModelInputBlur"
                @input="onModelInputChange"
                type="text"
                :placeholder="models.length ? 'Search models...' : 'Type model name or connect to browse'"
                class="w-full px-3 py-2 pr-8 text-sm rounded border border-[var(--border)] bg-[var(--bg-primary)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
              />
              <ChevronDown
                v-if="models.length > 0"
                class="w-4 h-4 absolute right-2 top-1/2 -translate-y-1/2 text-[var(--text-secondary)] pointer-events-none"
              />
            </div>

            <!-- Dropdown -->
            <div
              v-if="showModelDropdown && filteredModels.length > 0"
              class="absolute z-10 mt-1 w-full max-h-48 overflow-y-auto rounded border border-[var(--border)] bg-[var(--bg-surface)] shadow-lg"
            >
              <button
                v-for="m in filteredModels"
                :key="m.id"
                @mousedown.prevent="selectModel(m)"
                class="w-full text-left px-3 py-1.5 text-sm hover:bg-[var(--bg-hover)] transition-colors flex flex-col"
                :class="{ 'bg-[var(--bg-hover)]': m.id === model }"
              >
                <span class="text-[var(--text-primary)] truncate">{{ m.id }}</span>
                <span
                  v-if="m.name !== m.id"
                  class="text-xs text-[var(--text-secondary)] truncate"
                >{{ m.name }}</span>
              </button>
              <div v-if="filteredModels.length === 0" class="px-3 py-2 text-xs text-[var(--text-secondary)]">
                No models match "{{ modelSearch }}"
              </div>
            </div>
          </div>

          <!-- Read-only mode -->
          <div class="flex items-center justify-between py-1">
            <div>
              <label class="block text-xs text-[var(--text-primary)]">Read-only mode</label>
              <p class="text-xs text-[var(--text-secondary)] mt-0.5">Only allow informational queries — no VM actions</p>
            </div>
            <button
              @click="readOnly = !readOnly"
              class="relative w-9 h-5 rounded-full transition-colors"
              :class="readOnly ? 'bg-[var(--accent)]' : 'bg-[var(--bg-hover)]'"
            >
              <span
                class="absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform"
                :class="readOnly ? 'translate-x-4' : ''"
              />
            </button>
          </div>
        </div>

        <!-- Footer -->
        <div class="flex justify-end gap-2 px-4 py-3 border-t border-[var(--border)]">
          <button
            @click="emit('close')"
            class="px-4 py-1.5 text-sm rounded border border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors"
          >
            Cancel
          </button>
          <button
            @click="save"
            :disabled="saving"
            class="px-4 py-1.5 text-sm rounded bg-[var(--accent)] text-white hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {{ saving ? 'Saving...' : 'Save' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
