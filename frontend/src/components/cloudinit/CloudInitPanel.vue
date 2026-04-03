<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import CloudInitEditor from './CloudInitEditor.vue'
import ConfirmModal from '../modals/ConfirmModal.vue'
import ActionButton from '../shared/ActionButton.vue'
import { Plus, Trash2, Save, FileCode, AlertCircle, CheckCircle, Copy, Lock } from 'lucide-vue-next'
import { markRaw } from 'vue'

const SaveIcon = markRaw(Save)
const CopyIcon = markRaw(Copy)

const toasts = useToastStore()

const templates = ref([])
const selectedTemplate = ref(null)
const editorContent = ref('')
const originalContent = ref('')
const newFileName = ref('')
const isNew = ref(false)
const saving = ref(false)
const loading = ref(true)
const dirty = ref(false)
const showDeleteConfirm = ref(false)

const validation = ref({ valid: true, errors: [] })
const errorCount = computed(() => validation.value.errors.filter(e => e.severity === 'error').length)
const warningCount = computed(() => validation.value.errors.filter(e => e.severity === 'warning').length)

const isBuiltIn = computed(() => selectedTemplate.value?.builtIn && !isNew.value)
const isEditable = computed(() => isNew.value || (selectedTemplate.value && !selectedTemplate.value.builtIn))

const builtinTemplates = computed(() => templates.value.filter(t => t.builtIn))
const userTemplates = computed(() => templates.value.filter(t => !t.builtIn))

const DEFAULT_TEMPLATE = `#cloud-config

packages: []

runcmd: []
`

async function loadTemplates() {
  loading.value = true
  try {
    const result = await api.listCloudInitTemplates()
    templates.value = Array.isArray(result) ? result : []
  } catch {
    templates.value = []
  } finally {
    loading.value = false
  }
}

async function selectTemplate(tmpl) {
  if (dirty.value && !confirm('Discard unsaved changes?')) return
  try {
    const result = await api.getCloudInitTemplate(tmpl.label)
    selectedTemplate.value = { ...tmpl, builtIn: result.builtIn || tmpl.builtIn }
    editorContent.value = result.content
    originalContent.value = result.content
    isNew.value = false
    dirty.value = false
  } catch (e) {
    toasts.error(`Failed to load template: ${e.message}`)
  }
}

function startNew() {
  if (dirty.value && !confirm('Discard unsaved changes?')) return
  selectedTemplate.value = null
  editorContent.value = DEFAULT_TEMPLATE
  originalContent.value = DEFAULT_TEMPLATE
  newFileName.value = ''
  isNew.value = true
  dirty.value = false
}

function copyToEdit() {
  // Take the built-in content and start a new editable template from it
  const baseName = selectedTemplate.value.label.replace(/\.(yml|yaml)$/, '')
  newFileName.value = `${baseName}-custom.yml`
  isNew.value = true
  selectedTemplate.value = null
  dirty.value = true
}

function onContentChange(val) {
  editorContent.value = val
  dirty.value = val !== originalContent.value
}

function onValidation(v) {
  validation.value = v
}

async function save() {
  saving.value = true
  try {
    if (isNew.value) {
      let name = newFileName.value.trim()
      if (!name) {
        toasts.error('File name is required')
        return
      }
      if (!name.endsWith('.yml') && !name.endsWith('.yaml')) {
        name += '.yml'
      }
      await api.createCloudInitTemplate(name, editorContent.value)
      toasts.success(`Created ${name}`)
      isNew.value = false
      dirty.value = false
      await loadTemplates()
      const created = templates.value.find(t => t.label === name)
      if (created) selectedTemplate.value = created
    } else if (selectedTemplate.value && !selectedTemplate.value.builtIn) {
      await api.updateCloudInitTemplate(selectedTemplate.value.label, editorContent.value)
      toasts.success(`Saved ${selectedTemplate.value.label}`)
      dirty.value = false
    }
  } catch (e) {
    toasts.error(e.message)
  } finally {
    saving.value = false
  }
}

async function confirmDelete() {
  if (!selectedTemplate.value) return
  try {
    await api.deleteCloudInitTemplate(selectedTemplate.value.label)
    toasts.success(`Deleted ${selectedTemplate.value.label}`)
    selectedTemplate.value = null
    editorContent.value = ''
    isNew.value = false
    dirty.value = false
    await loadTemplates()
  } catch (e) {
    toasts.error(e.message)
  }
  showDeleteConfirm.value = false
}

onMounted(() => {
  loadTemplates()
  window.addEventListener('cloud-init-changed', loadTemplates)
})
onUnmounted(() => {
  window.removeEventListener('cloud-init-changed', loadTemplates)
})
</script>

<template>
  <div class="flex h-full">
    <!-- Template list sidebar -->
    <div class="w-64 flex-shrink-0 border-r border-[var(--border)] bg-[var(--bg-secondary)] flex flex-col">
      <div class="p-3 border-b border-[var(--border)] flex items-center justify-between">
        <h3 class="text-sm font-semibold">Cloud-Init Templates</h3>
        <button
          @click="startNew"
          class="p-1.5 rounded hover:bg-[var(--bg-hover)] transition-colors text-[var(--accent)]"
          title="New template"
        >
          <Plus class="w-4 h-4" />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto p-2 space-y-0.5">
        <div v-if="loading" class="text-xs text-[var(--text-secondary)] p-2">Loading...</div>
        <template v-else>
          <!-- Built-in templates -->
          <div v-if="builtinTemplates.length > 0">
            <div class="text-[10px] uppercase tracking-wider text-[var(--muted)] px-2 pt-2 pb-1">Built-in</div>
            <button
              v-for="tmpl in builtinTemplates"
              :key="tmpl.path"
              @click="selectTemplate(tmpl)"
              class="w-full flex items-center gap-2 px-2 py-1.5 rounded text-left text-sm transition-colors"
              :class="selectedTemplate?.path === tmpl.path && !isNew
                ? 'bg-[var(--accent)]/20 text-[var(--accent)]'
                : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
            >
              <Lock class="w-3 h-3 flex-shrink-0 opacity-50" />
              <span class="truncate">{{ tmpl.label }}</span>
            </button>
          </div>

          <!-- User templates -->
          <div v-if="userTemplates.length > 0">
            <div class="text-[10px] uppercase tracking-wider text-[var(--muted)] px-2 pt-3 pb-1">Custom</div>
            <button
              v-for="tmpl in userTemplates"
              :key="tmpl.path"
              @click="selectTemplate(tmpl)"
              class="w-full flex items-center gap-2 px-2 py-1.5 rounded text-left text-sm transition-colors"
              :class="selectedTemplate?.path === tmpl.path && !isNew
                ? 'bg-[var(--accent)]/20 text-[var(--accent)]'
                : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'"
            >
              <FileCode class="w-3.5 h-3.5 flex-shrink-0" />
              <span class="truncate">{{ tmpl.label }}</span>
            </button>
          </div>

          <div
            v-if="builtinTemplates.length === 0 && userTemplates.length === 0"
            class="text-xs text-[var(--text-secondary)] p-2"
          >
            No templates found.
          </div>
        </template>
      </div>
    </div>

    <!-- Editor area -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Toolbar -->
      <div class="flex items-center gap-3 px-4 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <template v-if="isNew">
          <input
            v-model="newFileName"
            type="text"
            placeholder="filename.yml"
            class="bg-[var(--bg-primary)] border border-[var(--border)] rounded px-2 py-1 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)] w-48"
          />
        </template>
        <template v-else-if="selectedTemplate">
          <span class="text-sm font-medium">{{ selectedTemplate.label }}</span>
          <span v-if="isBuiltIn" class="text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-[var(--muted)]/20 text-[var(--muted)]">read-only</span>
        </template>
        <template v-else>
          <span class="text-sm text-[var(--text-secondary)]">Select or create a template</span>
        </template>

        <div class="flex-1" />

        <!-- Validation badge -->
        <div v-if="isNew || selectedTemplate" class="flex items-center gap-1.5 text-xs">
          <template v-if="errorCount > 0">
            <AlertCircle class="w-3.5 h-3.5 text-[var(--danger)]" />
            <span class="text-[var(--danger)]">{{ errorCount }} error{{ errorCount !== 1 ? 's' : '' }}</span>
          </template>
          <template v-else-if="warningCount > 0">
            <AlertCircle class="w-3.5 h-3.5 text-[var(--warning)]" />
            <span class="text-[var(--warning)]">{{ warningCount }} warning{{ warningCount !== 1 ? 's' : '' }}</span>
          </template>
          <template v-else>
            <CheckCircle class="w-3.5 h-3.5 text-[var(--success)]" />
            <span class="text-[var(--success)]">Valid YAML</span>
          </template>
        </div>

        <!-- Actions -->
        <template v-if="isBuiltIn">
          <ActionButton
            @click="copyToEdit"
            :icon="CopyIcon"
            label="Copy to edit"
            variant="default"
          />
        </template>
        <template v-else-if="isNew || selectedTemplate">
          <ActionButton
            @click="save"
            :disabled="saving || (isNew && !newFileName.trim())"
            :icon="SaveIcon"
            :label="'Save' + (dirty ? '*' : '')"
            variant="default"
          />
          <button
            v-if="selectedTemplate && !isNew"
            @click="showDeleteConfirm = true"
            class="p-1.5 rounded hover:bg-[var(--danger)]/20 transition-colors text-[var(--danger)]"
            title="Delete template"
          >
            <Trash2 class="w-4 h-4" />
          </button>
        </template>
      </div>

      <!-- Editor -->
      <div v-if="isNew || selectedTemplate" class="flex-1 min-h-0">
        <CloudInitEditor
          :model-value="editorContent"
          :readonly="isBuiltIn"
          @update:model-value="onContentChange"
          @validation="onValidation"
        />
      </div>

      <!-- Empty state -->
      <div v-else class="flex-1 flex items-center justify-center text-[var(--text-secondary)]">
        <div class="text-center">
          <FileCode class="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p class="text-sm">Select a template to view or create a new one</p>
        </div>
      </div>
    </div>
  </div>

  <!-- Delete confirmation -->
  <ConfirmModal
    v-if="showDeleteConfirm"
    :message="`Delete '${selectedTemplate?.label}'? This cannot be undone.`"
    @confirm="confirmDelete"
    @cancel="showDeleteConfirm = false"
  />
</template>
