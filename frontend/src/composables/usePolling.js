import { ref, onMounted, onUnmounted } from 'vue'

export function createSerializedRunner(callback, isActive = () => true) {
  let current = null
  let rerunRequested = false

  function run() {
    if (!isActive()) return Promise.resolve()
    if (current) {
      rerunRequested = true
      return current
    }

    current = Promise.resolve()
      .then(callback)
      .finally(() => {
        current = null
        if (rerunRequested) {
          rerunRequested = false
          void run().catch(() => {})
        }
      })
    return current
  }

  return run
}

export function usePolling(callback, intervalMs = 3000) {
  const active = ref(true)
  let timer = null
  const run = createSerializedRunner(callback, () => active.value)

  function start() {
    if (timer) return
    timer = setInterval(() => {
      void run().catch(() => {})
    }, intervalMs)
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  function pause() { active.value = false }
  function resume() { active.value = true }

  function handleVisibility() {
    if (document.hidden) {
      pause()
    } else {
      resume()
      void run().catch(() => {})
    }
  }

  onMounted(() => {
    void run().catch(() => {})
    start()
    document.addEventListener('visibilitychange', handleVisibility)
  })

  onUnmounted(() => {
    stop()
    document.removeEventListener('visibilitychange', handleVisibility)
  })

  return { active, pause, resume, trigger: run }
}
