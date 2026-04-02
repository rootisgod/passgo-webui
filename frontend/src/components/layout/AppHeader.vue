<script setup>
import { useVmStore } from '../../stores/vmStore.js'
import { useChatStore } from '../../stores/chatStore.js'
import { logout } from '../../api/client.js'
import { Server, LogOut, MessageSquare } from 'lucide-vue-next'

const store = useVmStore()
const chatStore = useChatStore()

async function handleLogout() {
  try { await logout() } catch { /* ignore */ }
  store.authenticated = false
}
</script>

<template>
  <header class="flex items-center justify-between px-4 py-2 bg-[var(--bg-secondary)] border-b border-[var(--border)]">
    <div class="flex items-center gap-3">
      <Server class="w-5 h-5 text-[var(--accent)]" />
      <span class="font-semibold text-lg">PassGo Web</span>
    </div>
    <div class="flex items-center gap-4 text-sm text-[var(--text-secondary)]">
      <span>{{ store.hostname }}</span>
      <button
        @click="chatStore.togglePanel"
        class="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-[var(--bg-hover)] transition-colors"
        :class="{ 'text-[var(--accent)]': chatStore.isOpen }"
        title="AI Chat"
      >
        <MessageSquare class="w-4 h-4" />
      </button>
      <button
        @click="handleLogout"
        class="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-[var(--bg-hover)] transition-colors"
        title="Logout"
      >
        <LogOut class="w-4 h-4" />
      </button>
    </div>
  </header>
</template>
