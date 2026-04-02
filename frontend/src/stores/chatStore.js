import { defineStore } from 'pinia'
import { getChatConfig, updateChatConfig } from '../api/client.js'
import { useVmStore } from './vmStore.js'

// Tools that change VM state — trigger a VM list refresh after completion
const stateChangingTools = new Set([
  'start_vm', 'stop_vm', 'suspend_vm', 'delete_vm', 'recover_vm',
  'create_vm', 'create_snapshot', 'restore_snapshot', 'delete_snapshot',
  'exec_command',
  'create_group', 'rename_group', 'delete_group', 'assign_vm_to_group',
])

let nextId = 1

const STORAGE_KEY = 'passgo-chat-history'

function loadPersistedMessages() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const msgs = JSON.parse(raw)
    if (!Array.isArray(msgs)) return []
    // Restore nextId past any existing message IDs
    for (const m of msgs) {
      if (m.id >= nextId) nextId = m.id + 1
    }
    return msgs
  } catch {
    return []
  }
}

function persistMessages(messages) {
  try {
    // Only persist user/assistant messages, strip transient state
    const toSave = messages.map(m => ({
      id: m.id,
      role: m.role,
      content: m.content,
      toolEvents: m.toolEvents || undefined,
      isError: m.isError || undefined,
    }))
    localStorage.setItem(STORAGE_KEY, JSON.stringify(toSave))
  } catch {
    // localStorage full or unavailable — silently ignore
  }
}

export const useChatStore = defineStore('chat', {
  state: () => ({
    messages: loadPersistedMessages(),
    isOpen: false,
    isStreaming: false,
    config: {
      baseUrl: '',
      model: '',
      hasApiKey: false,
      readOnly: false,
    },
    pendingConfirmation: null,
    sessionUsage: { prompt: 0, completion: 0, total: 0 },
    error: null,
    abortController: null,
  }),

  actions: {
    togglePanel() {
      this.isOpen = !this.isOpen
      if (this.isOpen && !this.config.baseUrl) {
        this.loadConfig()
      }
    },
    openPanel() { this.isOpen = true },
    closePanel() { this.isOpen = false },

    clearHistory() {
      this.messages = []
      this.error = null
      this.sessionUsage = { prompt: 0, completion: 0, total: 0 }
      persistMessages(this.messages)
    },

    async loadConfig() {
      try {
        const cfg = await getChatConfig()
        this.config = {
          baseUrl: cfg.base_url,
          model: cfg.model,
          hasApiKey: cfg.has_api_key,
          readOnly: cfg.read_only,
        }
      } catch (e) {
        console.error('Failed to load chat config:', e)
      }
    },

    async saveConfig(cfg) {
      try {
        const result = await updateChatConfig({
          base_url: cfg.baseUrl,
          api_key: cfg.apiKey || '',
          model: cfg.model,
          read_only: cfg.readOnly,
        })
        this.config = {
          baseUrl: result.base_url,
          model: result.model,
          hasApiKey: result.has_api_key,
          readOnly: result.read_only,
        }
        return true
      } catch (e) {
        console.error('Failed to save chat config:', e)
        return false
      }
    },

    async sendMessage(text, confirmedTools = []) {
      if (!text.trim() || this.isStreaming) return

      // Only add user message if this isn't a confirmation retry
      if (confirmedTools.length === 0) {
        this.messages.push({
          id: nextId++,
          role: 'user',
          content: text.trim(),
        })
      }

      this.isStreaming = true
      this.error = null
      this.pendingConfirmation = null

      // Prepare history (only user/assistant messages for the LLM)
      const history = this.messages
        .filter(m => m.role === 'user' || m.role === 'assistant')
        .slice(-40)
        .map(m => ({ role: m.role, content: m.content }))

      // Add placeholder for assistant response (or reuse existing if retrying).
      // IMPORTANT: We must access the message through this.messages[idx] (the
      // reactive proxy), NOT through a local variable holding the raw object.
      // Mutating a local reference bypasses Vue's reactivity and the UI won't update.
      let msgIdx
      if (confirmedTools.length > 0) {
        msgIdx = this.messages.findLastIndex(m => m.role === 'assistant')
        if (msgIdx === -1) {
          this.messages.push({ id: nextId++, role: 'assistant', content: '', toolEvents: [] })
          msgIdx = this.messages.length - 1
        }
      } else {
        this.messages.push({
          id: nextId++,
          role: 'assistant',
          content: '',
          toolEvents: [],
        })
        msgIdx = this.messages.length - 1
      }

      const controller = new AbortController()
      this.abortController = controller

      try {
        const response = await fetch('/api/v1/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            message: text.trim(),
            history: history.slice(0, -1),
            confirmed_tools: confirmedTools,
          }),
          signal: controller.signal,
        })

        if (!response.ok) {
          const errBody = await response.text()
          let errMsg
          try { errMsg = JSON.parse(errBody).error } catch { errMsg = errBody }
          this.messages[msgIdx].content = `Error: ${errMsg}`
          this.messages[msgIdx].isError = true
          this.isStreaming = false
          return
        }

        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop()

          for (const line of lines) {
            if (!line.startsWith('data: ')) continue
            const payload = line.slice(6)

            let event
            try { event = JSON.parse(payload) } catch { continue }

            switch (event.type) {
              case 'token':
                this.messages[msgIdx].content += event.content
                break
              case 'tool_start':
                this.messages[msgIdx].toolEvents.push({
                  name: event.name,
                  args: event.args,
                  status: 'running',
                  result: null,
                })
                break
              case 'tool_done':
                {
                  const te = this.messages[msgIdx].toolEvents.find(
                    t => t.name === event.name && t.status === 'running'
                  )
                  if (te) {
                    te.status = 'done'
                    te.result = event.result
                  }
                  // Refresh VM list immediately after state-changing tools
                  if (stateChangingTools.has(event.name)) {
                    const vmStore = useVmStore()
                    vmStore.fetchVMs()
                  }
                }
                break
              case 'tool_progress':
                {
                  const te = this.messages[msgIdx].toolEvents.find(
                    t => t.name === event.name && t.status === 'running'
                  )
                  if (te) {
                    te.progress = event.content
                  }
                }
                break
              case 'confirm_required':
                {
                  const te = this.messages[msgIdx].toolEvents.find(
                    t => t.name === event.name && t.status === 'running'
                  )
                  if (te) {
                    te.status = 'pending_confirm'
                    te.result = event.description
                  }
                  this.pendingConfirmation = {
                    confirmId: event.confirm_id,
                    toolName: event.name,
                    description: event.description,
                    originalMessage: text.trim(),
                  }
                }
                break
              case 'error':
                this.messages[msgIdx].content += event.content
                this.messages[msgIdx].isError = true
                break
              case 'done':
                if (event.usage && event.usage.total_tokens > 0) {
                  this.sessionUsage.prompt += event.usage.prompt_tokens || 0
                  this.sessionUsage.completion += event.usage.completion_tokens || 0
                  this.sessionUsage.total += event.usage.total_tokens || 0
                }
                break
            }
          }
        }
      } catch (e) {
        if (e.name === 'AbortError') {
          this.messages[msgIdx].content += '\n[Cancelled]'
        } else {
          this.messages[msgIdx].content = `Connection error: ${e.message}`
          this.messages[msgIdx].isError = true
        }
      } finally {
        this.isStreaming = false
        this.abortController = null
        persistMessages(this.messages)
      }
    },

    confirmDestructiveAction() {
      if (!this.pendingConfirmation) return
      const { confirmId, originalMessage } = this.pendingConfirmation
      this.pendingConfirmation = null
      this.sendMessage(originalMessage, [confirmId])
    },

    denyDestructiveAction() {
      if (!this.pendingConfirmation) return
      this.pendingConfirmation = null
      const idx = this.messages.findLastIndex(m => m.role === 'assistant')
      if (idx !== -1) {
        const te = this.messages[idx].toolEvents.find(t => t.status === 'pending_confirm')
        if (te) {
          te.status = 'denied'
          te.result = 'Action cancelled by user'
        }
        this.messages[idx].content += '\nAction cancelled.'
      }
      persistMessages(this.messages)
    },

    cancelStream() {
      if (this.abortController) {
        this.abortController.abort()
        this.isStreaming = false
      }
    },
  },
})
