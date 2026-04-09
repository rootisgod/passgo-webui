<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { yaml } from '@codemirror/lang-yaml'
import { linter, lintGutter } from '@codemirror/lint'
import { bracketMatching, foldGutter, indentOnInput } from '@codemirror/language'
import { closeBrackets } from '@codemirror/autocomplete'
import { search, searchKeymap, highlightSelectionMatches } from '@codemirror/search'
import { indentationMarkers } from '@replit/codemirror-indentation-markers'
import YAML from 'js-yaml'
import { darkTheme, darkHighlightStyle } from '../cloudinit/editorTheme.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  readonly: { type: Boolean, default: false },
  fullscreen: { type: Boolean, default: false },
  wordWrap: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'validation', 'exit-fullscreen'])

const editorRef = ref(null)
let view = null
let destroyed = false
const wrapCompartment = new Compartment()

function yamlLinter() {
  return linter((editorView) => {
    if (destroyed) return []
    const content = editorView.state.doc.toString()
    const doc = editorView.state.doc
    const diagnostics = []

    try {
      YAML.load(content)
    } catch (e) {
      if (e.mark) {
        const pos = doc.line(Math.min(e.mark.line + 1, doc.lines))
        diagnostics.push({
          from: pos.from,
          to: pos.to,
          severity: 'error',
          message: e.reason || e.message,
        })
      } else {
        diagnostics.push({
          from: 0,
          to: 0,
          severity: 'error',
          message: e.message,
        })
      }
    }

    if (!destroyed) {
      emit('validation', {
        valid: diagnostics.filter(d => d.severity === 'error').length === 0,
        errors: diagnostics,
      })
    }

    return diagnostics
  }, { delay: 300 })
}

onMounted(() => {
  const extensions = [
    lineNumbers(),
    highlightActiveLine(),
    highlightActiveLineGutter(),
    history(),
    foldGutter(),
    indentOnInput(),
    indentationMarkers(),
    bracketMatching(),
    closeBrackets(),
    yaml(),
    darkTheme,
    darkHighlightStyle,
    lintGutter(),
    yamlLinter(),
    wrapCompartment.of(props.wordWrap ? EditorView.lineWrapping : []),
    search(),
    highlightSelectionMatches(),
    keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
    keymap.of([{
      key: 'Escape',
      run: () => {
        if (props.fullscreen) { emit('exit-fullscreen'); return true }
        return false
      },
    }]),
    EditorView.updateListener.of((update) => {
      if (update.docChanged && !destroyed) {
        emit('update:modelValue', update.state.doc.toString())
      }
    }),
  ]

  if (props.readonly) {
    extensions.push(EditorState.readOnly.of(true))
  }

  view = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions,
    }),
    parent: editorRef.value,
  })
})

watch(() => props.fullscreen, () => {
  if (view) view.requestMeasure()
})

watch(() => props.wordWrap, (val) => {
  if (view) {
    view.dispatch({ effects: wrapCompartment.reconfigure(val ? EditorView.lineWrapping : []) })
  }
})

watch(() => props.modelValue, (newVal) => {
  if (view && !destroyed && newVal !== view.state.doc.toString()) {
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: newVal },
    })
  }
})

onUnmounted(() => {
  destroyed = true
  if (view) {
    view.destroy()
    view = null
  }
})
</script>

<template>
  <div
    class="overflow-hidden"
    :class="fullscreen ? 'fixed inset-0 z-30 flex flex-col bg-[#1a1a2e]' : 'h-full rounded border border-[var(--border)]'"
  >
    <div v-if="fullscreen" class="flex items-center justify-between px-3 py-1.5 bg-[#16213e] border-b border-[#334155] flex-shrink-0">
      <span class="text-xs text-[#94a3b8]">Fullscreen Editor</span>
      <button
        @click="emit('exit-fullscreen')"
        class="px-3 py-1 text-xs rounded bg-[#2a3a5c] text-[#e2e8f0] border border-[#334155] hover:bg-[#3b82f6] transition-colors"
      >
        Exit Fullscreen
      </button>
    </div>
    <div ref="editorRef" class="flex-1 min-h-0 overflow-auto" />
  </div>
</template>
