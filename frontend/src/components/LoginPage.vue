<script setup>
import { ref } from 'vue'
import { useVmStore } from '../stores/vmStore.js'
import { useToastStore } from '../stores/toastStore.js'
import { login, getVersion } from '../api/client.js'
import { Server, Loader2 } from 'lucide-vue-next'

const store = useVmStore()
const toasts = useToastStore()
const username = ref('')
const password = ref('')
const submitting = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  submitting.value = true
  try {
    await login(username.value, password.value)
    store.authenticated = true
    // Fetch hostname
    try {
      const ver = await getVersion()
      store.hostname = ver.hostname || 'localhost'
    } catch { /* ok */ }
  } catch (e) {
    error.value = e.status === 401 ? 'Invalid credentials' : e.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-[var(--bg-primary)]">
    <div class="bg-[var(--bg-surface)] rounded-xl border border-[var(--border)] p-8 w-full max-w-sm shadow-2xl">
      <div class="flex items-center justify-center gap-3 mb-8">
        <Server class="w-8 h-8 text-[var(--accent)]" />
        <h1 class="text-2xl font-bold">PassGo Web</h1>
      </div>

      <form @submit.prevent="submit" class="space-y-4">
        <div>
          <label class="block text-xs text-[var(--text-secondary)] mb-1">Username</label>
          <input
            v-model="username"
            type="text"
            autocomplete="username"
            class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
            autofocus
          />
        </div>
        <div>
          <label class="block text-xs text-[var(--text-secondary)] mb-1">Password</label>
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            class="w-full bg-[var(--bg-primary)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]"
          />
        </div>

        <div v-if="error" class="text-sm text-[var(--danger)]">{{ error }}</div>

        <button
          type="submit"
          :disabled="submitting || !username || !password"
          class="w-full flex items-center justify-center gap-2 py-2 rounded bg-[var(--accent)] hover:bg-blue-600 transition-colors text-sm font-medium disabled:opacity-40"
        >
          <Loader2 v-if="submitting" class="w-4 h-4 animate-spin" />
          Log in
        </button>
      </form>
    </div>
  </div>
</template>
