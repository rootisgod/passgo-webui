const API_BASE = '/api/v1'

class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
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
export const getCloudInitStatus = (name) => request('GET', `/vms/${name}/cloud-init/status`)

// Snapshots
export const listSnapshots = (vmName) => request('GET', `/vms/${vmName}/snapshots`)
export const createSnapshot = (vmName, name, comment) => request('POST', `/vms/${vmName}/snapshots`, { name, comment })
export const restoreSnapshot = (vmName, snap) => request('POST', `/vms/${vmName}/snapshots/${snap}/restore`)
export const deleteSnapshot = (vmName, snap) => request('DELETE', `/vms/${vmName}/snapshots/${snap}`)

// Mounts
export const listMounts = (vmName) => request('GET', `/vms/${vmName}/mounts`)
export const addMount = (vmName, source, target) => request('POST', `/vms/${vmName}/mounts`, { source, target })
export const removeMount = (vmName, target) => request('DELETE', `/vms/${vmName}/mounts`, { target })

// System
export const listNetworks = () => request('GET', '/networks')
export const listCloudInitTemplates = () => request('GET', '/cloud-init/templates')
export const getCloudInitTemplate = (name) => request('GET', `/cloud-init/templates/${encodeURIComponent(name)}`)
export const createCloudInitTemplate = (name, content) => request('POST', '/cloud-init/templates', { name, content })
export const updateCloudInitTemplate = (name, content) => request('PUT', `/cloud-init/templates/${encodeURIComponent(name)}`, { content })
export const deleteCloudInitTemplate = (name) => request('DELETE', `/cloud-init/templates/${encodeURIComponent(name)}`)
export const getVersion = () => request('GET', '/version')

export { ApiError }
