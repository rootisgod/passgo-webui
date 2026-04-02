<script setup>
import { ref, computed, onMounted } from 'vue'
import { X, ChevronDown, Loader2, Check, AlertCircle } from 'lucide-vue-next'
import { useChatStore } from '../../stores/chatStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import { listChatModels } from '../../api/client.js'

const emit = defineEmits(['close'])
const chatStore = useChatStore()
const toasts = useToastStore()

const baseUrl = ref('')
const apiKey = ref('')
const model = ref('')
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

// Whether we can fetch models (have credentials or it's local)
const canConnect = computed(() => {
  if (!baseUrl.value) return false
  if (isLocal.value) return true
  return !!(apiKey.value || chatStore.config.hasApiKey)
})

onMounted(() => {
  baseUrl.value = chatStore.config.baseUrl
  model.value = chatStore.config.model
  readOnly.value = chatStore.config.readOnly
  modelSearch.value = model.value

  // Auto-fetch only if already configured with working credentials
  if (chatStore.config.hasApiKey || isLocal.value) {
    connectAndFetchModels()
  }
})

async function connectAndFetchModels() {
  if (!baseUrl.value) return
  testing.value = true
  modelsLoading.value = true
  modelsError.value = null
  connectionTested.value = false

  try {
    // Save URL + key so the backend proxy can use them
    await chatStore.saveConfig({
      baseUrl: baseUrl.value,
      apiKey: apiKey.value || '',
      model: model.value || chatStore.config.model || '',
      readOnly: readOnly.value,
    })

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

  if (preset === 'openrouter') {
    baseUrl.value = 'https://openrouter.ai/api/v1'
    if (!model.value) {
      model.value = 'anthropic/claude-sonnet-4'
      modelSearch.value = model.value
    }
  } else if (preset === 'ollama') {
    baseUrl.value = 'http://localhost:11434/v1'
    if (!model.value) {
      model.value = 'llama3.2'
      modelSearch.value = model.value
    }
    // Ollama is local — connect automatically
    connectAndFetchModels()
  }
}

async function save() {
  saving.value = true
  const ok = await chatStore.saveConfig({
    baseUrl: baseUrl.value,
    apiKey: apiKey.value,
    model: model.value,
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
            <div class="flex gap-2">
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

          <!-- Base URL -->
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1.5">Base URL</label>
            <input
              v-model="baseUrl"
              type="text"
              placeholder="https://openrouter.ai/api/v1"
              class="w-full px-3 py-2 text-sm rounded border border-[var(--border)] bg-[var(--bg-primary)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
            />
          </div>

          <!-- API Key + Connect button -->
          <div>
            <label class="block text-xs text-[var(--text-secondary)] mb-1.5">
              API Key
              <span v-if="chatStore.config.hasApiKey && !apiKey" class="text-green-400 ml-1">(configured)</span>
            </label>
            <div class="flex gap-2">
              <input
                v-model="apiKey"
                type="password"
                :placeholder="chatStore.config.hasApiKey ? '••••••••' : 'Not set'"
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
              {{ modelsError || (isLocal ? 'Not required for local providers' : 'Enter key and click Connect to browse models') }}
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
