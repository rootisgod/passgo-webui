<script setup>
import { ref } from 'vue'
import { useVmStore } from '../stores/vmStore.js'
import { login } from '../api/client.js'
import { Server } from 'lucide-vue-next'

const store = useVmStore()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await login(username.value, password.value)
    store.authenticated = true
    store.fetchVMs()
  } catch (e) {
    error.value = e.message || 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="h-screen flex items-center justify-center bg-[var(--bg-primary)]">
    <form @submit.prevent="handleLogin" class="w-80 p-6 rounded-lg bg-[var(--bg-surface)] border border-[var(--border)]">
      <div class="flex items-center justify-center gap-3 mb-6">
        <Server class="w-6 h-6 text-[var(--accent)]" />
        <span class="text-xl font-semibold">PassGo Web</span>
      </div>

      <div v-if="error" class="mb-4 p-2 rounded text-sm bg-red-900/30 text-red-300 border border-red-800">
        {{ error }}
      </div>

      <label class="block mb-1 text-sm text-[var(--text-secondary)]">Username</label>
      <input
        v-model="username"
        type="text"
        autocomplete="username"
        class="w-full mb-4 px-3 py-2 rounded bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
      />

      <label class="block mb-1 text-sm text-[var(--text-secondary)]">Password</label>
      <input
        v-model="password"
        type="password"
        autocomplete="current-password"
        class="w-full mb-6 px-3 py-2 rounded bg-[var(--bg-primary)] border border-[var(--border)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
      />

      <button
        type="submit"
        :disabled="loading || !username || !password"
        class="w-full py-2 rounded font-medium bg-[var(--accent)] text-white hover:opacity-90 transition-opacity disabled:opacity-40"
      >
        {{ loading ? 'Signing in...' : 'Sign In' }}
      </button>
    </form>
  </div>
</template>
