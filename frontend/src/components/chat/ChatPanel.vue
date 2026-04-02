<script setup>
import { ref, watch, nextTick, onMounted } from 'vue'
import { X, Settings, Trash2, Send, Square, Bot, ShieldAlert, Eye } from 'lucide-vue-next'
import { useChatStore } from '../../stores/chatStore.js'
import ChatMessage from './ChatMessage.vue'
import ChatSettingsModal from './ChatSettingsModal.vue'

const chatStore = useChatStore()
const inputText = ref('')
const messagesEnd = ref(null)
const showSettings = ref(false)

onMounted(() => {
  chatStore.loadConfig()
})

// Auto-scroll when messages change
watch(
  () => chatStore.messages.length > 0
    ? chatStore.messages[chatStore.messages.length - 1].content
    : null,
  () => {
    nextTick(() => {
      messagesEnd.value?.scrollIntoView({ behavior: 'smooth' })
    })
  },
)

// Also scroll when tool events appear
watch(
  () => {
    const msgs = chatStore.messages
    if (msgs.length === 0) return 0
    const last = msgs[msgs.length - 1]
    return (last.toolEvents?.length || 0) + (last.content?.length || 0)
  },
  () => {
    nextTick(() => {
      messagesEnd.value?.scrollIntoView({ behavior: 'smooth' })
    })
  },
)

function handleKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

function send() {
  const text = inputText.value.trim()
  if (!text) return
  if (chatStore.isStreaming) {
    chatStore.cancelStream()
  }
  inputText.value = ''
  chatStore.sendMessage(text)
}
</script>

<template>
  <Transition name="slide">
    <div
      v-if="chatStore.isOpen"
      class="flex flex-col h-full w-[400px] border-l border-[var(--border)] bg-[var(--bg-primary)] flex-shrink-0"
    >
      <!-- Header -->
      <div class="flex items-center justify-between px-3 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <div class="flex items-center gap-2">
          <Bot class="w-4 h-4 text-[var(--accent)]" />
          <span class="text-sm font-medium">AI Assistant</span>
          <span
            v-if="chatStore.config.readOnly"
            class="flex items-center gap-1 px-1.5 py-0.5 text-[10px] rounded bg-yellow-900/40 text-yellow-300 border border-yellow-800/50"
            title="Read-only mode — actions are disabled"
          >
            <Eye class="w-3 h-3" />
            Read-only
          </span>
        </div>
        <div class="flex items-center gap-1">
          <button
            @click="chatStore.clearHistory"
            class="p-1.5 rounded hover:bg-[var(--bg-hover)] transition-colors"
            title="Clear chat"
          >
            <Trash2 class="w-3.5 h-3.5 text-[var(--text-secondary)]" />
          </button>
          <button
            @click="showSettings = true"
            class="p-1.5 rounded hover:bg-[var(--bg-hover)] transition-colors"
            title="Chat settings"
          >
            <Settings class="w-3.5 h-3.5 text-[var(--text-secondary)]" />
          </button>
          <button
            @click="chatStore.closePanel"
            class="p-1.5 rounded hover:bg-[var(--bg-hover)] transition-colors"
            title="Close chat"
          >
            <X class="w-3.5 h-3.5 text-[var(--text-secondary)]" />
          </button>
        </div>
      </div>

      <!-- Messages -->
      <div class="flex-1 overflow-y-auto py-3">
        <!-- Empty state -->
        <div v-if="chatStore.messages.length === 0" class="flex flex-col items-center justify-center h-full text-center px-6">
          <Bot class="w-10 h-10 text-[var(--text-secondary)] mb-3 opacity-40" />
          <p class="text-sm text-[var(--text-secondary)]">
            Ask me about your VMs or tell me to perform actions like starting, stopping, or creating instances.
          </p>
          <p class="text-xs text-[var(--text-secondary)] mt-2 opacity-60">
            Configure your LLM provider in
            <button @click="showSettings = true" class="underline hover:text-[var(--text-primary)]">settings</button>
          </p>
        </div>

        <ChatMessage
          v-for="msg in chatStore.messages"
          :key="msg.id"
          :message="msg"
        />
        <div ref="messagesEnd" />
      </div>

      <!-- Destructive action confirmation -->
      <div
        v-if="chatStore.pendingConfirmation"
        class="mx-3 mb-2 p-3 rounded border border-amber-700/60 bg-amber-950/40"
      >
        <div class="flex items-start gap-2 mb-2">
          <ShieldAlert class="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
          <div>
            <p class="text-sm font-medium text-amber-300">Confirm action</p>
            <p class="text-xs text-[var(--text-secondary)] mt-0.5">
              {{ chatStore.pendingConfirmation.description }}
            </p>
          </div>
        </div>
        <div class="flex gap-2 justify-end">
          <button
            @click="chatStore.denyDestructiveAction"
            class="px-3 py-1 text-xs rounded border border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors"
          >
            Cancel
          </button>
          <button
            @click="chatStore.confirmDestructiveAction"
            class="px-3 py-1 text-xs rounded bg-red-700 text-white hover:bg-red-600 transition-colors"
          >
            Confirm
          </button>
        </div>
      </div>

      <!-- Input -->
      <div class="border-t border-[var(--border)] p-3">
        <div class="flex gap-2">
          <textarea
            v-model="inputText"
            @keydown="handleKeydown"
            placeholder="Ask about your VMs..."
            rows="1"
            class="flex-1 px-3 py-2 text-sm rounded border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] placeholder-[var(--text-secondary)] resize-none focus:outline-none focus:border-[var(--accent)]"
          />
          <button
            v-if="chatStore.isStreaming"
            @click="chatStore.cancelStream"
            class="p-2 rounded bg-red-600 text-white hover:bg-red-700 transition-colors flex-shrink-0"
            title="Stop"
          >
            <Square class="w-4 h-4" />
          </button>
          <button
            @click="send"
            :disabled="!inputText.trim()"
            class="p-2 rounded bg-[var(--accent)] text-white hover:opacity-90 transition-opacity disabled:opacity-30 flex-shrink-0"
            title="Send"
          >
            <Send class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </Transition>

  <ChatSettingsModal v-if="showSettings" @close="showSettings = false" />
</template>

<style scoped>
.slide-enter-active,
.slide-leave-active {
  transition: width 0.25s ease, opacity 0.25s ease;
}
.slide-enter-from,
.slide-leave-to {
  width: 0;
  opacity: 0;
  overflow: hidden;
}
</style>
