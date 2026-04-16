import { ref } from 'vue'

// Generic file-list state + navigation. Caller provides a fetcher(path)
// that returns the entries array — used by the VM file browser (multipass
// exec ls) and the host file browser (os.ReadDir on the server).
// Entries share a common shape: { name, size, permissions, modified, isDir }.
export function useFileList(fetcher, initialPath = '/') {
  const currentPath = ref(initialPath)
  const pathInput = ref(initialPath)
  const files = ref([])
  const loading = ref(false)
  const error = ref('')

  async function loadFiles() {
    loading.value = true
    error.value = ''
    try {
      const data = await fetcher(currentPath.value)
      files.value = Array.isArray(data) ? data : []
      pathInput.value = currentPath.value
    } catch (e) {
      files.value = []
      error.value = e.message || 'Failed to list files'
    } finally {
      loading.value = false
    }
  }

  // Path joining/parent logic handles both POSIX (/home/ubuntu) and Windows
  // (C:\Users\me) style paths for the host browser. The VM side is always
  // POSIX, so this is a no-op there.
  function isWindowsPath(p) {
    return p.includes('\\') && !p.startsWith('/')
  }

  function joinPath(base, name) {
    const sep = isWindowsPath(base) ? '\\' : '/'
    // Treat empty/sep-only base as root so we never produce "//child".
    const trimmed = base.replace(/[\\/]+$/, '')
    if (!trimmed) return sep + name
    return trimmed + sep + name
  }

  function parentOf(p) {
    const sep = isWindowsPath(p) ? '\\' : '/'
    const re = isWindowsPath(p) ? /\\[^\\]+[\\]?$/ : /\/[^/]+\/?$/
    const parent = p.replace(re, '')
    return parent || sep
  }

  function navigateTo(dirName) {
    currentPath.value = joinPath(currentPath.value, dirName)
    loadFiles()
  }

  function goUp() {
    currentPath.value = parentOf(currentPath.value)
    loadFiles()
  }

  function goToPath() {
    currentPath.value = pathInput.value || '/'
    loadFiles()
  }

  function reset(path) {
    currentPath.value = path
    pathInput.value = path
    files.value = []
    error.value = ''
  }

  return {
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
  }
}
