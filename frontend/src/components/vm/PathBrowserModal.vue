<script setup>
import { computed, ref, watch } from 'vue'
import { useFileList } from '../../composables/useFileList.js'
import { Folder, FolderPlus, File as FileIcon, ArrowUp, RefreshCw, X, Check, PowerOff } from 'lucide-vue-next'

const props = defineProps({
  isOpen: { type: Boolean, default: false },
  fetcher: { type: Function, required: true },
  initialPath: { type: String, default: '/' },
  directoriesOnly: { type: Boolean, default: false },
  title: { type: String, default: 'Browse filesystem' },
  // When set, the modal renders this reason instead of the file list. Use for
  // "VM is not running" or any other pre-condition the caller wants to gate on.
  disabledReason: { type: String, default: '' },
  // Optional async callback invoked when the user clicks "New Folder" and
  // confirms a name: createFolder(fullPath) should return a promise. When
  // provided, the modal renders a "+ New Folder" control; when absent, it
  // doesn't. Host-side browsing deliberately doesn't pass this in.
  createFolder: { type: Function, default: null },
})

const emit = defineEmits(['close', 'select'])

const {
  currentPath,
  pathInput,
  files,
  loading,
  error,
  loadFiles,
  navigateTo,
  goUp,
  goToPath,
  reset,
} = useFileList(props.fetcher, props.initialPath)

const visibleFiles = computed(() => {
  if (!props.directoriesOnly) return files.value
  return files.value.filter((f) => f.isDir)
})

const enabled = computed(() => !props.disabledReason)

const newFolderMode = ref(false)
const newFolderName = ref('')
const creating = ref(false)

watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      reset(props.initialPath)
      newFolderMode.value = false
      newFolderName.value = ''
      if (enabled.value) loadFiles()
    }
  },
)

function onSelectCurrent() {
  emit('select', currentPath.value)
}

function openNewFolder() {
  newFolderMode.value = true
  newFolderName.value = ''
}

function cancelNewFolder() {
  newFolderMode.value = false
  newFolderName.value = ''
}

async function confirmNewFolder() {
  const name = newFolderName.value.trim()
  if (!name) return
  // Single segment only — avoid slashes, dots-only, and traversal sequences.
  if (name === '.' || name === '..' || /[\/\\]/.test(name)) {
    error.value = 'Folder name must not contain / or \\'
    return
  }
  const base = currentPath.value.replace(/\/+$/, '') || ''
  const fullPath = base === '' ? '/' + name : base + '/' + name
  creating.value = true
  try {
    await props.createFolder(fullPath)
    newFolderMode.value = false
    newFolderName.value = ''
    await loadFiles()
    // Auto-navigate into the newly created folder — usually what the user wants
    // when creating a mount target they're about to select.
    currentPath.value = fullPath
    pathInput.value = fullPath
    await loadFiles()
  } catch (e) {
    error.value = e.message || 'Failed to create folder'
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="isOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      @click.self="emit('close')"
    >
      <div
        class="bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg shadow-xl w-[720px] max-w-[90vw] max-h-[80vh] flex flex-col"
      >
        <!-- Header -->
        <div class="flex items-center justify-between px-5 py-3 border-b border-[var(--border)]">
          <h3 class="text-base font-semibold">{{ title }}</h3>
          <button
            @click="emit('close')"
            class="p-1 rounded hover:bg-[var(--bg-hover)] transition-colors"
            title="Close"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Body -->
        <div class="flex-1 flex flex-col overflow-hidden">
          <div
            v-if="!enabled"
            class="flex flex-col items-center justify-center gap-3 py-12 text-[var(--text-secondary)]"
          >
            <PowerOff class="w-10 h-10 text-[var(--muted)]" />
            <p class="text-base">{{ disabledReason }}</p>
          </div>

          <template v-else>
            <!-- Path bar -->
            <div class="flex items-center gap-2 px-5 pt-4 pb-3">
              <button
                @click="goUp"
                class="p-2 rounded hover:bg-[var(--bg-hover)] transition-colors"
                title="Parent directory"
              >
                <ArrowUp class="w-4 h-4" />
              </button>
              <input
                v-model="pathInput"
                @keyup.enter="goToPath"
                class="flex-1 bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm font-mono focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
              />
              <button
                @click="goToPath"
                class="px-3 py-1.5 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
              >Go</button>
              <button
                @click="loadFiles"
                class="p-2 rounded hover:bg-[var(--bg-hover)] transition-colors"
                title="Refresh"
              >
                <RefreshCw class="w-4 h-4" />
              </button>
              <button
                v-if="createFolder && !newFolderMode"
                @click="openNewFolder"
                class="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
                title="Create a new folder in this directory"
              >
                <FolderPlus class="w-4 h-4" />
                New Folder
              </button>
            </div>

            <!-- Inline new-folder input -->
            <div
              v-if="newFolderMode"
              class="flex items-center gap-2 px-5 pb-3"
            >
              <input
                v-model="newFolderName"
                @keyup.enter="confirmNewFolder"
                @keyup.escape="cancelNewFolder"
                placeholder="New folder name"
                autofocus
                class="flex-1 bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
              />
              <button
                @click="confirmNewFolder"
                :disabled="!newFolderName.trim() || creating"
                class="px-3 py-1.5 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
              >{{ creating ? 'Creating...' : 'Create' }}</button>
              <button
                @click="cancelNewFolder"
                :disabled="creating"
                class="px-3 py-1.5 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
              >Cancel</button>
            </div>

            <!-- Error -->
            <div
              v-if="error"
              class="mx-5 mb-3 px-3 py-2 text-sm rounded border border-[var(--danger)] bg-red-900/20 text-red-300"
            >
              {{ error }}
            </div>

            <!-- File listing -->
            <div class="flex-1 overflow-y-auto px-5 pb-3">
              <div
                v-if="loading"
                class="flex items-center justify-center py-8 text-[var(--text-secondary)]"
              >
                <RefreshCw class="w-4 h-4 animate-spin mr-2" />
                Loading...
              </div>
              <div
                v-else-if="visibleFiles.length === 0"
                class="flex flex-col items-center py-8 text-[var(--text-secondary)]"
              >
                <Folder class="w-8 h-8 mb-2 text-[var(--muted)]" />
                <p class="text-sm">
                  {{ directoriesOnly ? 'No subdirectories here' : 'Empty directory' }}
                </p>
              </div>
              <table v-else class="w-full text-sm">
                <tbody>
                  <tr
                    v-for="entry in visibleFiles"
                    :key="entry.name"
                    class="border-b border-[var(--border)] last:border-b-0 hover:bg-[var(--bg-hover)]"
                  >
                    <td class="px-2 py-2">
                      <button
                        v-if="entry.isDir"
                        @click="navigateTo(entry.name)"
                        class="flex items-center gap-2 text-[var(--accent)] hover:underline"
                      >
                        <Folder class="w-4 h-4" />
                        {{ entry.name }}
                      </button>
                      <span v-else class="flex items-center gap-2 text-[var(--text-secondary)]">
                        <FileIcon class="w-4 h-4" />
                        {{ entry.name }}
                      </span>
                    </td>
                    <td class="px-2 py-2 font-mono text-xs text-[var(--text-secondary)] whitespace-nowrap">
                      {{ entry.permissions }}
                    </td>
                    <td class="px-2 py-2 text-xs text-[var(--text-secondary)] whitespace-nowrap">
                      {{ entry.modified }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </template>
        </div>

        <!-- Footer -->
        <div class="flex items-center justify-between px-5 py-3 border-t border-[var(--border)]">
          <div class="text-xs text-[var(--text-secondary)] font-mono truncate">
            <span v-if="enabled">Selected: {{ currentPath }}</span>
          </div>
          <div class="flex gap-2">
            <button
              @click="emit('close')"
              class="px-3 py-1.5 text-sm rounded bg-[var(--bg-hover)] hover:bg-[var(--border)] transition-colors"
            >Cancel</button>
            <button
              @click="onSelectCurrent"
              :disabled="!enabled"
              class="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
            >
              <Check class="w-4 h-4" />
              Select This Folder
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
