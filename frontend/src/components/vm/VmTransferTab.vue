<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useVmStore } from '../../stores/vmStore.js'
import { useToastStore } from '../../stores/toastStore.js'
import * as api from '../../api/client.js'
import { useFileList } from '../../composables/useFileList.js'
import { Folder, File, Upload, Download, ArrowUp, RefreshCw, PowerOff, Play } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const vm = computed(() => store.selectedVm)
const isRunning = computed(() => vm.value?.state === 'Running')
const isDeleted = computed(() => vm.value?.state === 'Deleted')
const starting = ref(false)

async function powerOn() {
  starting.value = true
  try {
    await api.startVM(store.selectedNode)
    toasts.success(`${store.selectedNode} starting...`)
    store.fetchVMs()
  } catch (e) {
    toasts.error(e.message)
  } finally {
    starting.value = false
  }
}

const {
  currentPath,
  pathInput,
  files,
  loading,
  error: listError,
  loadFiles,
  navigateTo,
  goUp,
  goToPath,
} = useFileList((path) => api.listFiles(store.selectedNode, path), '/home/ubuntu')

watch(listError, (msg) => { if (msg) toasts.error(msg) })

const uploading = ref(false)
const dragging = ref(false)
const fileInputRef = ref(null)

async function handleDownload(fileName) {
  const remotePath = currentPath.value.replace(/\/$/, '') + '/' + fileName
  try {
    await api.downloadFile(store.selectedNode, remotePath)
    toasts.success(`Downloaded ${fileName}`)
  } catch (e) {
    toasts.error(e.message)
  }
}

async function doUpload(file) {
  uploading.value = true
  try {
    await api.uploadFile(store.selectedNode, currentPath.value, file)
    toasts.success(`Uploaded ${file.name}`)
    loadFiles()
  } catch (e) {
    toasts.error(e.message)
  } finally {
    uploading.value = false
  }
}

function handleFileSelect(event) {
  const file = event.target.files[0]
  if (file) doUpload(file)
  event.target.value = ''
}

function handleDrop(event) {
  dragging.value = false
  const file = event.dataTransfer.files[0]
  if (file) doUpload(file)
}

function formatSize(sizeStr) {
  const bytes = parseInt(sizeStr, 10)
  if (isNaN(bytes)) return sizeStr
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
}

watch(isRunning, (running) => {
  if (running && files.value.length === 0) loadFiles()
})

onMounted(() => {
  if (isRunning.value) loadFiles()
})
</script>

<template>
  <!-- VM deleted -->
  <div v-if="isDeleted" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
    <PowerOff class="w-12 h-12 text-[var(--muted)]" />
    <p class="text-lg">VM Deleted</p>
    <p class="text-sm">Recover this VM to browse files</p>
  </div>

  <!-- VM not running -->
  <div v-else-if="!isRunning" class="flex flex-col items-center justify-center h-full gap-4 text-[var(--text-secondary)]">
    <PowerOff class="w-12 h-12 text-[var(--muted)]" />
    <p class="text-lg">Powered Off</p>
    <p class="text-sm">Start the VM to browse files</p>
    <button
      @click="powerOn"
      :disabled="starting"
      class="flex items-center gap-2 mt-2 px-4 py-2 text-sm rounded bg-green-900/30 hover:bg-[var(--success)] text-green-300 hover:text-white transition-colors disabled:opacity-40"
    >
      <Play class="w-4 h-4" />
      {{ starting ? 'Starting...' : 'Start VM' }}
    </button>
  </div>

  <!-- VM running -->
  <div v-else class="p-6">
    <!-- Path bar -->
    <div class="flex items-center gap-2 mb-4">
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
        class="flex-1 bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-1.5 text-sm font-mono text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
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
        @click="fileInputRef.click()"
        :disabled="uploading"
        class="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors disabled:opacity-40"
      >
        <Upload class="w-4 h-4" />
        {{ uploading ? 'Uploading...' : 'Upload' }}
      </button>
      <input ref="fileInputRef" type="file" class="hidden" @change="handleFileSelect" />
    </div>

    <!-- Drop zone + file table -->
    <div
      @dragover.prevent="dragging = true"
      @dragleave="dragging = false"
      @drop.prevent="handleDrop"
      class="bg-[var(--bg-surface)] rounded-lg border overflow-hidden transition-colors"
      :class="dragging ? 'border-[var(--accent)] bg-blue-900/10' : 'border-[var(--border)]'"
    >
      <!-- Drop overlay -->
      <div v-if="dragging" class="flex items-center justify-center py-12 text-[var(--accent)]">
        <Upload class="w-8 h-8 mr-3" />
        <span class="text-lg">Drop file to upload to {{ currentPath }}</span>
      </div>

      <!-- File table -->
      <table v-else-if="files.length > 0" class="w-full text-sm">
        <thead>
          <tr class="border-b border-[var(--border)] text-[var(--text-secondary)]">
            <th class="text-left px-4 py-2.5 font-medium">Name</th>
            <th class="text-left px-4 py-2.5 font-medium">Size</th>
            <th class="text-left px-4 py-2.5 font-medium">Permissions</th>
            <th class="text-left px-4 py-2.5 font-medium">Modified</th>
            <th class="text-right px-4 py-2.5 font-medium">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="entry in files"
            :key="entry.name"
            class="border-b border-[var(--border)] last:border-b-0 hover:bg-[var(--bg-hover)]"
          >
            <td class="px-4 py-2.5">
              <button
                v-if="entry.isDir"
                @click="navigateTo(entry.name)"
                class="flex items-center gap-2 text-[var(--accent)] hover:underline"
              >
                <Folder class="w-4 h-4" />
                {{ entry.name }}
              </button>
              <span v-else class="flex items-center gap-2">
                <File class="w-4 h-4 text-[var(--text-secondary)]" />
                {{ entry.name }}
              </span>
            </td>
            <td class="px-4 py-2.5 text-[var(--text-secondary)]">{{ entry.isDir ? '—' : formatSize(entry.size) }}</td>
            <td class="px-4 py-2.5 font-mono text-xs text-[var(--text-secondary)]">{{ entry.permissions }}</td>
            <td class="px-4 py-2.5 text-[var(--text-secondary)]">{{ entry.modified }}</td>
            <td class="px-4 py-2.5 text-right">
              <button
                v-if="!entry.isDir"
                @click="handleDownload(entry.name)"
                class="p-1 rounded hover:bg-[var(--accent)] transition-colors"
                title="Download"
              >
                <Download class="w-4 h-4" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Empty state -->
      <div v-else-if="!loading" class="flex flex-col items-center justify-center py-12 text-[var(--text-secondary)]">
        <Folder class="w-10 h-10 mb-2 text-[var(--muted)]" />
        <p>Empty directory</p>
        <p class="text-xs mt-1">Drop a file here or click Upload</p>
      </div>

      <!-- Loading -->
      <div v-else class="flex items-center justify-center py-12 text-[var(--text-secondary)]">
        <RefreshCw class="w-5 h-5 animate-spin mr-2" />
        Loading...
      </div>
    </div>
  </div>
</template>
