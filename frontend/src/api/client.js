const API_BASE = '/api/v1'

class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

// Fired when any API call sees 401. App.vue listens and routes back to login,
// so no individual caller needs to handle auth-expiry. Skip firing for the
// login endpoint itself — that's an expected path for bad credentials.
function fireUnauthorized(path) {
  if (path === '/auth/login') return
  window.dispatchEvent(new CustomEvent('passgo:unauthorized'))
}

async function request(method, path, body) {
  const opts = {
    method,
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) {
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(API_BASE + path, opts)
  const text = await res.text()
  let data
  try {
    data = JSON.parse(text)
  } catch {
    data = { message: text }
  }
  if (res.status === 401) {
    fireUnauthorized(path)
  }
  if (!res.ok) {
    throw new ApiError(res.status, data.error || text)
  }
  return data
}

// VMs
export const listVMs = () => request('GET', '/vms')
export const getVM = (name) => request('GET', `/vms/${name}`)
export const createVM = (opts) => request('POST', '/vms', opts)
export const startVM = (name) => request('POST', `/vms/${name}/start`)
export const stopVM = (name) => request('POST', `/vms/${name}/stop`)
export const suspendVM = (name) => request('POST', `/vms/${name}/suspend`)
export const deleteVM = (name, purge = false) => request('DELETE', `/vms/${name}`, { purge })
export const recoverVM = (name) => request('POST', `/vms/${name}/recover`)
export const startAll = () => request('POST', '/vms/start-all')
export const stopAll = () => request('POST', '/vms/stop-all')
export const purgeDeleted = () => request('POST', '/vms/purge')
export const execInVM = (name, command) => request('POST', `/vms/${name}/exec`, { command })
export const listLaunches = () => request('GET', '/launches')
export const dismissLaunch = (name) => request('DELETE', `/launches/${encodeURIComponent(name)}`)
export const cloneVM = (name, destName, snapshot) => request('POST', `/vms/${name}/clone`, { name: destName, snapshot })
export const getCloudInitStatus = (name) => request('GET', `/vms/${name}/cloud-init/status`)
export const getVMConfig = (name) => request('GET', `/vms/${name}/config`)
export const resizeVM = (name, config) => request('PUT', `/vms/${name}/config`, config)
export const getHostResources = () => request('GET', '/host/resources')
export const getVMDefaults = () => request('GET', '/config/vm-defaults')
export const updateVMDefaults = (defaults) => request('PUT', '/config/vm-defaults', defaults)

// Config export/import
export async function exportConfig() {
  const res = await fetch(API_BASE + '/config/export')
  if (res.status === 401) fireUnauthorized('/config/export')
  if (!res.ok) {
    const text = await res.text()
    let msg
    try { msg = JSON.parse(text).error } catch { msg = text }
    throw new ApiError(res.status, msg)
  }
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `passgo-config-${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
}
export const importConfig = (bundle) => request('POST', '/config/import', bundle)

// Snapshots
export const listSnapshots = (vmName) => request('GET', `/vms/${vmName}/snapshots`)
export const createSnapshot = (vmName, name, comment) => request('POST', `/vms/${vmName}/snapshots`, { name, comment })
export const restoreSnapshot = (vmName, snap) => request('POST', `/vms/${vmName}/snapshots/${snap}/restore`)
export const deleteSnapshot = (vmName, snap) => request('DELETE', `/vms/${vmName}/snapshots/${snap}`)

// Mounts
export const listMounts = (vmName) => request('GET', `/vms/${vmName}/mounts`)
export const addMount = (vmName, source, target) => request('POST', `/vms/${vmName}/mounts`, { source, target })
export const removeMount = (vmName, target) => request('DELETE', `/vms/${vmName}/mounts`, { target })
export const openMountFolder = (vmName, target) => request('POST', `/vms/${vmName}/mounts/open`, { target })

// System
export const listImages = () => request('GET', '/images')
export const listNetworks = () => request('GET', '/networks')
export const listCloudInitTemplates = () => request('GET', '/cloud-init/templates')
export const getCloudInitTemplate = (name) => request('GET', `/cloud-init/templates/${encodeURIComponent(name)}`)
export const createCloudInitTemplate = (name, content) => request('POST', '/cloud-init/templates', { name, content })
export const updateCloudInitTemplate = (name, content) => request('PUT', `/cloud-init/templates/${encodeURIComponent(name)}`, { content })
export const deleteCloudInitTemplate = (name) => request('DELETE', `/cloud-init/templates/${encodeURIComponent(name)}`)
export const getVersion = () => request('GET', '/version')

// File transfer
export const listFiles = (vmName, path) => request('GET', `/vms/${vmName}/files/ls?path=${encodeURIComponent(path)}`)
export const listHostFiles = (path) => request('GET', `/host/files/ls${path ? `?path=${encodeURIComponent(path)}` : ''}`)
export const getHostHome = () => request('GET', '/host/home')
export const createVmFolder = (vmName, path) => request('POST', `/vms/${vmName}/files/mkdir`, { path })

export async function uploadFile(vmName, destPath, file) {
  const form = new FormData()
  form.append('file', file)
  form.append('path', destPath)
  const res = await fetch(`${API_BASE}/vms/${vmName}/files`, { method: 'POST', body: form })
  if (res.status === 401) fireUnauthorized(`/vms/${vmName}/files`)
  if (!res.ok) {
    const text = await res.text()
    let msg
    try { msg = JSON.parse(text).error } catch { msg = text }
    throw new ApiError(res.status, msg)
  }
}

export async function downloadFile(vmName, remotePath) {
  const res = await fetch(`${API_BASE}/vms/${vmName}/files?path=${encodeURIComponent(remotePath)}`)
  if (res.status === 401) fireUnauthorized(`/vms/${vmName}/files`)
  if (!res.ok) {
    const text = await res.text()
    let msg
    try { msg = JSON.parse(text).error } catch { msg = text }
    throw new ApiError(res.status, msg)
  }
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = remotePath.split('/').pop()
  a.click()
  URL.revokeObjectURL(url)
}

// Groups
export const listGroups = () => request('GET', '/groups')
export const createGroup = (name) => request('POST', '/groups', { name })
export const renameGroup = (name, newName) => request('PUT', `/groups/${encodeURIComponent(name)}`, { name: newName })
export const deleteGroup = (name) => request('DELETE', `/groups/${encodeURIComponent(name)}`)
export const assignVmGroup = (vm, group) => request('PUT', '/groups/assign', { vm, group })
export const reorderGroups = (groups) => request('PUT', '/groups/reorder', { groups })

// Ansible playbooks
export const getAnsibleStatus = () => request('GET', '/ansible/status')
export const listPlaybooks = () => request('GET', '/ansible/playbooks')
export const getPlaybook = (name) => request('GET', `/ansible/playbooks/${encodeURIComponent(name)}`)
export const createPlaybook = (name, content) => request('POST', '/ansible/playbooks', { name, content })
export const updatePlaybook = (name, content) => request('PUT', `/ansible/playbooks/${encodeURIComponent(name)}`, { content })
export const deletePlaybook = (name) => request('DELETE', `/ansible/playbooks/${encodeURIComponent(name)}`)
export const getAnsibleRunStatus = () => request('GET', '/ansible/run/status')
export const cancelAnsibleRun = () => request('DELETE', '/ansible/run')
export const clearAnsibleRun = () => request('POST', '/ansible/run/clear')

// Profiles
export const listProfiles = () => request('GET', '/profiles')
export const createProfile = (profile) => request('POST', '/profiles', profile)
export const updateProfile = (id, profile) => request('PUT', `/profiles/${encodeURIComponent(id)}`, profile)
export const deleteProfile = (id) => request('DELETE', `/profiles/${encodeURIComponent(id)}`)

// Ansible queue
export const getAnsibleRunQueue = () => request('GET', '/ansible/run/queue')
export const clearAnsibleRunQueue = () => request('DELETE', '/ansible/run/queue')

// Schedules
export const listSchedules = () => request('GET', '/schedules')
export const createSchedule = (schedule) => request('POST', '/schedules', schedule)
export const updateSchedule = (id, schedule) => request('PUT', `/schedules/${encodeURIComponent(id)}`, schedule)
export const deleteSchedule = (id) => request('DELETE', `/schedules/${encodeURIComponent(id)}`)
export const runScheduleNow = (id) => request('POST', `/schedules/${encodeURIComponent(id)}/run`)
export const getScheduleHistory = () => request('GET', '/schedules/history')

// API Tokens
export const listTokens = () => request('GET', '/tokens')
export const createToken = (name) => request('POST', '/tokens', { name })
export const deleteToken = (id) => request('DELETE', `/tokens/${encodeURIComponent(id)}`)

// Webhooks
export const listWebhooks = () => request('GET', '/webhooks')
export const createWebhook = (webhook) => request('POST', '/webhooks', webhook)
export const updateWebhook = (id, webhook) => request('PUT', `/webhooks/${encodeURIComponent(id)}`, webhook)
export const deleteWebhook = (id) => request('DELETE', `/webhooks/${encodeURIComponent(id)}`)
export const testWebhook = (id) => request('POST', `/webhooks/${encodeURIComponent(id)}/test`)

// Event log
export const getEvents = (params = {}) => {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v != null && v !== '') qs.set(k, String(v))
  }
  const query = qs.toString()
  return request('GET', '/events' + (query ? '?' + query : ''))
}

// Shell sessions
export const createShellSession = (vmName) => request('POST', `/vms/${vmName}/shell/sessions`)
export const listShellSessions = (vmName) => request('GET', `/vms/${vmName}/shell/sessions`)
export const deleteShellSession = (vmName, sessionId) => request('DELETE', `/vms/${vmName}/shell/sessions/${sessionId}`)

// Chat / LLM
export const getChatConfig = () => request('GET', '/chat/config')
export const updateChatConfig = (cfg) => request('PUT', '/chat/config', cfg)
export const listChatModels = () => request('GET', '/chat/models')

// Auth
export const login = (username, password) => request('POST', '/auth/login', { username, password })
export const logout = () => request('POST', '/auth/logout')

export { ApiError }
