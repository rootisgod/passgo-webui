import { ref } from 'vue'

export function useWebSocket(vmName) {
  const connected = ref(false)
  const error = ref(null)
  let ws = null

  function getWsUrl(name) {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${location.host}/api/v1/vms/${name}/shell`
  }

  function connect(name, onData) {
    disconnect()
    error.value = null

    const url = getWsUrl(name || vmName)
    ws = new WebSocket(url)
    ws.binaryType = 'arraybuffer'

    ws.onopen = () => { connected.value = true }

    ws.onmessage = (event) => {
      if (onData) {
        const data = event.data instanceof ArrayBuffer
          ? new Uint8Array(event.data)
          : new TextEncoder().encode(event.data)
        onData(data)
      }
    }

    ws.onclose = () => { connected.value = false }

    ws.onerror = (e) => {
      error.value = 'Connection failed'
      connected.value = false
    }
  }

  function send(data) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  }

  function sendResize(cols, rows) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      const buf = new Uint8Array(5)
      buf[0] = 1 // resize prefix
      buf[1] = (cols >> 8) & 0xff
      buf[2] = cols & 0xff
      buf[3] = (rows >> 8) & 0xff
      buf[4] = rows & 0xff
      ws.send(buf)
    }
  }

  function disconnect() {
    if (ws) {
      ws.close()
      ws = null
    }
    connected.value = false
  }

  return { connected, error, connect, send, sendResize, disconnect }
}
