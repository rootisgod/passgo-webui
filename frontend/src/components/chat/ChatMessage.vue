<script setup>
import { computed } from 'vue'
import { Wrench, AlertCircle, User, Bot, ShieldAlert, Ban } from 'lucide-vue-next'
import { renderMarkdown } from '../../composables/useMarkdown.js'

const props = defineProps({
  message: { type: Object, required: true },
})

const renderedContent = computed(() => {
  if (props.message.role === 'assistant' && !props.message.isError) {
    return renderMarkdown(props.message.content)
  }
  return null
})

// Use blocks for interleaved rendering when available, else fall back to legacy layout
const hasBlocks = computed(() => props.message.blocks && props.message.blocks.length > 0)

// Pre-render markdown for each text block
const renderedBlocks = computed(() => {
  if (!hasBlocks.value) return []
  return props.message.blocks.map(block => {
    if (block.type === 'text' && props.message.role === 'assistant' && !props.message.isError) {
      return { ...block, rendered: renderMarkdown(block.content) }
    }
    return block
  })
})

function formatToolName(name) {
  return name.replace(/_/g, ' ')
}

function summarizeResult(result) {
  if (!result) return ''
  if (result.length > 120) return result.slice(0, 120) + '...'
  return result
}
</script>

<template>
  <div class="flex gap-2.5 px-3 py-2" :class="message.role === 'user' ? 'justify-end' : 'justify-start'">
    <!-- Avatar -->
    <div v-if="message.role !== 'user'" class="flex-shrink-0 w-7 h-7 rounded-full bg-[var(--bg-hover)] flex items-center justify-center mt-0.5">
      <Bot class="w-4 h-4 text-[var(--accent)]" />
    </div>

    <div class="max-w-[85%] flex flex-col gap-1.5">
      <!-- Interleaved blocks layout (new messages) -->
      <template v-if="hasBlocks">
        <template v-for="(block, i) in renderedBlocks" :key="i">
          <!-- Tool block -->
          <div
            v-if="block.type === 'tool'"
            class="flex items-center gap-1.5 text-xs px-2 py-1 rounded"
            :class="{
              'bg-[var(--bg-surface)] text-[var(--text-secondary)]': block.status === 'running' || block.status === 'done',
              'bg-amber-950/40 text-amber-300 border border-amber-800/50': block.status === 'pending_confirm',
              'bg-red-950/30 text-red-400 border border-red-800/40': block.status === 'denied',
            }"
          >
            <ShieldAlert v-if="block.status === 'pending_confirm'" class="w-3 h-3 flex-shrink-0" />
            <Ban v-else-if="block.status === 'denied'" class="w-3 h-3 flex-shrink-0" />
            <Wrench v-else class="w-3 h-3 flex-shrink-0" />
            <span class="font-medium">{{ formatToolName(block.name) }}</span>
            <span v-if="block.status === 'running' && block.progress" class="truncate opacity-70 animate-pulse" :title="block.progress">{{ block.progress }}</span>
            <span v-else-if="block.status === 'running'" class="animate-pulse">...</span>
            <span v-else-if="block.status === 'pending_confirm'" class="text-amber-400">awaiting confirmation</span>
            <span v-else-if="block.status === 'denied'" class="opacity-70">cancelled</span>
            <span v-else-if="block.result" class="truncate opacity-70" :title="block.result">
              {{ summarizeResult(block.result) }}
            </span>
          </div>
          <!-- Text block -->
          <div
            v-else-if="block.type === 'text' && block.content"
            class="rounded-lg px-3 py-2 text-sm leading-relaxed"
            :class="{
              'bg-[var(--accent)] text-white': message.role === 'user',
              'bg-[var(--bg-surface)] text-[var(--text-primary)]': message.role === 'assistant' && !message.isError,
              'bg-red-900/30 text-red-300 border border-red-800/50': message.isError,
            }"
          >
            <div v-if="block.rendered" class="md-content break-words" v-html="block.rendered" />
            <span v-else class="whitespace-pre-wrap break-words">{{ block.content }}</span>
          </div>
        </template>
      </template>

      <!-- Legacy layout (persisted messages without blocks) -->
      <template v-else>
        <!-- Tool events -->
        <div v-if="message.toolEvents?.length" class="flex flex-col gap-1">
          <div
            v-for="(te, i) in message.toolEvents"
            :key="i"
            class="flex items-center gap-1.5 text-xs px-2 py-1 rounded"
            :class="{
              'bg-[var(--bg-surface)] text-[var(--text-secondary)]': te.status === 'running' || te.status === 'done',
              'bg-amber-950/40 text-amber-300 border border-amber-800/50': te.status === 'pending_confirm',
              'bg-red-950/30 text-red-400 border border-red-800/40': te.status === 'denied',
            }"
          >
            <ShieldAlert v-if="te.status === 'pending_confirm'" class="w-3 h-3 flex-shrink-0" />
            <Ban v-else-if="te.status === 'denied'" class="w-3 h-3 flex-shrink-0" />
            <Wrench v-else class="w-3 h-3 flex-shrink-0" />
            <span class="font-medium">{{ formatToolName(te.name) }}</span>
            <span v-if="te.status === 'running' && te.progress" class="truncate opacity-70 animate-pulse" :title="te.progress">{{ te.progress }}</span>
            <span v-else-if="te.status === 'running'" class="animate-pulse">...</span>
            <span v-else-if="te.status === 'pending_confirm'" class="text-amber-400">awaiting confirmation</span>
            <span v-else-if="te.status === 'denied'" class="opacity-70">cancelled</span>
            <span v-else-if="te.result" class="truncate opacity-70" :title="te.result">
              {{ summarizeResult(te.result) }}
            </span>
          </div>
        </div>

        <!-- Message bubble -->
        <div
          v-if="message.content"
          class="rounded-lg px-3 py-2 text-sm leading-relaxed"
          :class="{
            'bg-[var(--accent)] text-white': message.role === 'user',
            'bg-[var(--bg-surface)] text-[var(--text-primary)]': message.role === 'assistant' && !message.isError,
            'bg-red-900/30 text-red-300 border border-red-800/50': message.isError,
          }"
        >
          <div class="flex items-start gap-1.5" v-if="message.isError">
            <AlertCircle class="w-4 h-4 flex-shrink-0 mt-0.5" />
            <span class="whitespace-pre-wrap break-words">{{ message.content }}</span>
          </div>
          <div v-else-if="renderedContent" class="md-content break-words" v-html="renderedContent" />
          <span v-else class="whitespace-pre-wrap break-words">{{ message.content }}</span>
        </div>
      </template>
    </div>

    <!-- User avatar -->
    <div v-if="message.role === 'user'" class="flex-shrink-0 w-7 h-7 rounded-full bg-[var(--accent)] flex items-center justify-center mt-0.5">
      <User class="w-4 h-4 text-white" />
    </div>
  </div>
</template>

<style scoped>
.md-content :deep(.md-code-block) {
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
  margin: 0.375rem 0;
  overflow-x: auto;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: 0.8125rem;
  white-space: pre;
}

.md-content :deep(.md-inline-code) {
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 0.25rem;
  padding: 0.1rem 0.35rem;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: 0.8125rem;
}

.md-content :deep(.md-list) {
  padding-left: 1.25rem;
  margin: 0.25rem 0;
}

.md-content :deep(ul.md-list) {
  list-style: disc;
}

.md-content :deep(ol.md-list) {
  list-style: decimal;
}

.md-content :deep(li) {
  margin: 0.125rem 0;
}

.md-content :deep(strong) {
  font-weight: 600;
}

.md-content :deep(em) {
  font-style: italic;
}

.md-content {
  white-space: pre-wrap;
}
</style>
