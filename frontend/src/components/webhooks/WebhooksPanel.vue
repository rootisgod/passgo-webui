<script setup>
import { ref, onMounted } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { Plus, Pencil, Trash2, Save, X, Send } from 'lucide-vue-next'

const toasts = useToastStore()

const CATEGORIES = [
  { value: 'vm', label: 'VM', color: 'bg-blue-500/20 text-blue-400' },
  { value: 'schedule', label: 'Schedule', color: 'bg-purple-500/20 text-purple-400' },
  { value: 'ansible', label: 'Ansible', color: 'bg-orange-500/20 text-orange-400' },
  { value: 'llm', label: 'LLM', color: 'bg-emerald-500/20 text-emerald-400' },
  { value: 'config', label: 'Config', color: 'bg-gray-500/20 text-gray-400' },
]

const RESULTS = [
  { value: 'success', label: 'Success' },
  { value: 'failed', label: 'Failed' },
  { value: 'partial', label: 'Partial' },
  { value: 'no_targets', label: 'No Targets' },
]

const webhooks = ref([])
const loading = ref(true)
const editing = ref(null)
const saving = ref(false)
const testing = ref(null)

// Form state
const formName = ref('')
const formUrl = ref('')
const formEnabled = ref(true)
const formCategories = ref([])
const formResults = ref([])
const formSecret = ref('')

onMounted(async () => {
  try {
    const data = await api.listWebhooks()
    webhooks.value = Array.isArray(data) ? data : []
  } catch { /* ignore */ }
  loading.value = false
})

function startNew() {
  editing.value = '__new__'
  formName.value = ''
  formUrl.value = ''
  formEnabled.value = true
  formCategories.value = []
  formResults.value = []
  formSecret.value = ''
}

function startEdit(wh) {
  editing.value = wh.id
  formName.value = wh.name
  formUrl.value = wh.url
  formEnabled.value = wh.enabled
  formCategories.value = [...(wh.categories || [])]
  formResults.value = [...(wh.results || [])]
  formSecret.value = ''
}

function cancelEdit() {
  editing.value = null
}

function toggleCategory(cat) {
  const idx = formCategories.value.indexOf(cat)
  if (idx >= 0) formCategories.value.splice(idx, 1)
  else formCategories.value.push(cat)
}

function toggleResult(result) {
  const idx = formResults.value.indexOf(result)
  if (idx >= 0) formResults.value.splice(idx, 1)
  else formResults.value.push(result)
}

function canSave() {
  return formName.value.trim() !== '' && formUrl.value.trim() !== ''
}

async function saveWebhook() {
  saving.value = true
  try {
    const data = {
      name: formName.value.trim(),
      url: formUrl.value.trim(),
      enabled: formEnabled.value,
      categories: formCategories.value,
      results: formResults.value,
    }
    if (formSecret.value) {
      data.secret = formSecret.value
    }
    if (editing.value === '__new__') {
      await api.createWebhook(data)
      toasts.success(`Webhook "${formName.value}" created`)
    } else {
      await api.updateWebhook(editing.value, data)
      toasts.success(`Webhook "${formName.value}" updated`)
    }
    webhooks.value = await api.listWebhooks()
    editing.value = null
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}

async function deleteWebhook(id, name) {
  if (!confirm(`Delete webhook "${name}"?`)) return
  try {
    await api.deleteWebhook(id)
    toasts.success(`Webhook "${name}" deleted`)
    if (editing.value === id) editing.value = null
    webhooks.value = await api.listWebhooks()
  } catch (e) {
    toasts.error(e.message)
  }
}

async function testWebhookFn(wh) {
  testing.value = wh.id
  try {
    await api.testWebhook(wh.id)
    toasts.success(`Test sent to "${wh.name}"`)
  } catch (e) {
    toasts.error(e.message)
  } finally {
    testing.value = null
  }
}

async function toggleEnabled(wh) {
  try {
    await api.updateWebhook(wh.id, { ...wh, enabled: !wh.enabled, secret: undefined })
    webhooks.value = await api.listWebhooks()
  } catch (e) {
    toasts.error(e.message)
  }
}

function categoryColor(cat) {
  const c = CATEGORIES.find(c => c.value === cat)
  return c ? c.color : 'bg-gray-500/20 text-gray-400'
}

function categoryLabel(cat) {
  const c = CATEGORIES.find(c => c.value === cat)
  return c ? c.label : cat
}

function truncateUrl(url) {
  if (url.length <= 40) return url
  return url.substring(0, 40) + '...'
}
</script>

<template>
  <div class="h-full flex flex-col">
    <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
      <h2 class="text-lg font-semibold">Webhooks</h2>
      <button
        @click="startNew"
        :disabled="editing === '__new__'"
        class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
      >
        <Plus class="w-3.5 h-3.5" />
        New Webhook
      </button>
    </div>

    <div v-if="loading" class="flex-1 flex items-center justify-center text-[var(--text-secondary)] text-sm">
      Loading...
    </div>

    <div v-else class="flex-1 overflow-y-auto">
      <!-- Empty state -->
      <div v-if="webhooks.length === 0 && editing !== '__new__'" class="p-6 text-center text-[var(--text-secondary)] text-sm">
        <p>No webhooks configured.</p>
        <p class="mt-1">Webhooks send HTTP POST notifications when events occur (VM stopped, schedule failed, etc.).</p>
      </div>

      <div class="divide-y divide-[var(--border)]">
        <!-- New webhook form -->
        <div v-if="editing === '__new__'" class="p-4 bg-[var(--bg-primary)]/50">
          <div class="space-y-3">
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Name</label>
              <input v-model="formName" type="text" placeholder="e.g. Notify on failures"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
            </div>

            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">URL</label>
              <input v-model="formUrl" type="url" placeholder="https://ntfy.sh/my-topic"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
            </div>

            <!-- Categories -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Categories <span class="text-[var(--text-secondary)]">(empty = all)</span></label>
              <div class="flex gap-1.5 flex-wrap">
                <button v-for="cat in CATEGORIES" :key="cat.value"
                  @click="toggleCategory(cat.value)"
                  class="px-2 py-1 text-xs rounded transition-colors"
                  :class="formCategories.includes(cat.value) ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                >{{ cat.label }}</button>
              </div>
            </div>

            <!-- Results -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Results <span class="text-[var(--text-secondary)]">(empty = all)</span></label>
              <div class="flex gap-1.5 flex-wrap">
                <button v-for="res in RESULTS" :key="res.value"
                  @click="toggleResult(res.value)"
                  class="px-2 py-1 text-xs rounded transition-colors"
                  :class="formResults.includes(res.value) ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                >{{ res.label }}</button>
              </div>
            </div>

            <!-- Secret -->
            <div>
              <label class="block text-xs text-[var(--text-secondary)] mb-1">Secret <span class="text-[var(--text-secondary)]">(optional, for HMAC-SHA256 signing)</span></label>
              <input v-model="formSecret" type="password" placeholder="Leave empty for no signing"
                class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
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
              <button @click="saveWebhook" :disabled="!canSave() || saving"
                class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40">
                <Save class="w-3.5 h-3.5" /> Create
              </button>
            </div>
          </div>
        </div>

        <!-- Existing webhooks -->
        <div v-for="wh in webhooks" :key="wh.id">
          <!-- Edit mode -->
          <div v-if="editing === wh.id" class="p-4 bg-[var(--bg-primary)]/50">
            <div class="space-y-3">
              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Name</label>
                <input v-model="formName" type="text"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">URL</label>
                <input v-model="formUrl" type="url"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Categories <span class="text-[var(--text-secondary)]">(empty = all)</span></label>
                <div class="flex gap-1.5 flex-wrap">
                  <button v-for="cat in CATEGORIES" :key="cat.value"
                    @click="toggleCategory(cat.value)"
                    class="px-2 py-1 text-xs rounded transition-colors"
                    :class="formCategories.includes(cat.value) ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                  >{{ cat.label }}</button>
                </div>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Results <span class="text-[var(--text-secondary)]">(empty = all)</span></label>
                <div class="flex gap-1.5 flex-wrap">
                  <button v-for="res in RESULTS" :key="res.value"
                    @click="toggleResult(res.value)"
                    class="px-2 py-1 text-xs rounded transition-colors"
                    :class="formResults.includes(res.value) ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'"
                  >{{ res.label }}</button>
                </div>
              </div>

              <div>
                <label class="block text-xs text-[var(--text-secondary)] mb-1">Secret <span class="text-[var(--text-secondary)]">(leave empty to keep existing)</span></label>
                <input v-model="formSecret" type="password" placeholder="Leave empty to keep existing"
                  class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2.5 py-1.5 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]" />
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
                <button @click="saveWebhook" :disabled="!canSave() || saving"
                  class="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40">
                  <Save class="w-3.5 h-3.5" /> Save
                </button>
              </div>
            </div>
          </div>

          <!-- View mode -->
          <div v-else class="flex items-center gap-3 px-4 py-3 hover:bg-[var(--bg-hover)] transition-colors">
            <button @click="toggleEnabled(wh)"
              class="w-8 h-4 rounded-full transition-colors relative flex-shrink-0"
              :class="wh.enabled ? 'bg-[var(--accent)]' : 'bg-[var(--border)]'"
              :title="wh.enabled ? 'Disable' : 'Enable'"
            >
              <span class="absolute top-0.5 w-3 h-3 rounded-full bg-white transition-transform"
                :class="wh.enabled ? 'left-4' : 'left-0.5'" />
            </button>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium" :class="wh.enabled ? 'text-[var(--text-primary)]' : 'text-[var(--text-secondary)]'">{{ wh.name }}</div>
              <div class="text-xs text-[var(--text-secondary)] truncate">{{ truncateUrl(wh.url) }}</div>
              <div v-if="wh.categories?.length || wh.results?.length" class="flex gap-1 mt-1 flex-wrap">
                <span v-for="cat in (wh.categories || [])" :key="cat"
                  class="px-1.5 py-0.5 text-[10px] rounded" :class="categoryColor(cat)">{{ categoryLabel(cat) }}</span>
                <span v-for="res in (wh.results || [])" :key="res"
                  class="px-1.5 py-0.5 text-[10px] rounded bg-[var(--bg-primary)] text-[var(--text-secondary)]">{{ res }}</span>
              </div>
            </div>
            <button @click="testWebhookFn(wh)" :disabled="testing === wh.id" title="Send Test"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--success)] disabled:opacity-40">
              <Send class="w-3.5 h-3.5" />
            </button>
            <button @click="startEdit(wh)"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)]">
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button @click="deleteWebhook(wh.id, wh.name)"
              class="p-1.5 rounded hover:bg-[var(--bg-primary)] transition-colors text-[var(--text-secondary)] hover:text-[var(--danger)]">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
