import { ref, onMounted, onUnmounted } from 'vue'

export function usePolling(callback, intervalMs = 3000) {
  const active = ref(true)
  let timer = null

  function start() {
    if (timer) return
    timer = setInterval(() => {
      if (active.value) callback()
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
      callback() // immediate refresh on return
    }
  }

  onMounted(() => {
    callback() // initial fetch
    start()
    document.addEventListener('visibilitychange', handleVisibility)
  })

  onUnmounted(() => {
    stop()
    document.removeEventListener('visibilitychange', handleVisibility)
  })

  return { active, pause, resume, trigger: callback }
}
