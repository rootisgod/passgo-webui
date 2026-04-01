import { reactive } from 'vue'

// Max samples: ~1 hour at 3s polling = 1200 points
const MAX_SAMPLES = 1200

// Shared reactive store keyed by VM name
const history = reactive({})

export function recordMetrics(vmName, metrics) {
  if (!history[vmName]) {
    history[vmName] = { cpu: [], memory: [], disk: [] }
  }
  const h = history[vmName]
  h.cpu.push(metrics.cpu)
  h.memory.push(metrics.memory)
  h.disk.push(metrics.disk)

  if (h.cpu.length > MAX_SAMPLES) h.cpu.shift()
  if (h.memory.length > MAX_SAMPLES) h.memory.shift()
  if (h.disk.length > MAX_SAMPLES) h.disk.shift()
}

export function getHistory(vmName) {
  return history[vmName] || { cpu: [], memory: [], disk: [] }
}

export function clearHistory(vmName) {
  delete history[vmName]
}
