<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { yaml } from '@codemirror/lang-yaml'
import { linter, lintGutter } from '@codemirror/lint'
import { bracketMatching, foldGutter, indentOnInput } from '@codemirror/language'
import { closeBrackets } from '@codemirror/autocomplete'
import YAML from 'js-yaml'
import { darkTheme, darkHighlightStyle } from './editorTheme.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  readonly: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'validation'])

const editorRef = ref(null)
let view = null
let destroyed = false

function yamlLinter() {
  return linter((editorView) => {
    if (destroyed) return []
    const content = editorView.state.doc.toString()
    const diagnostics = []

    // Check #cloud-config header
    const firstLine = content.split('\n')[0]?.trim()
    if (firstLine !== '#cloud-config') {
      diagnostics.push({
        from: 0,
        to: Math.min(content.length, content.indexOf('\n') > 0 ? content.indexOf('\n') : content.length),
        severity: 'warning',
        message: 'First line should be "#cloud-config"',
      })
    }

    // Parse YAML
    try {
      YAML.load(content)
    } catch (e) {
      if (e.mark) {
        const pos = editorView.state.doc.line(Math.min(e.mark.line + 1, editorView.state.doc.lines))
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
    bracketMatching(),
    closeBrackets(),
    yaml(),
    darkTheme,
    darkHighlightStyle,
    lintGutter(),
    yamlLinter(),
    keymap.of([...defaultKeymap, ...historyKeymap]),
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

// Watch for external content changes (e.g. loading a different template)
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
  <div ref="editorRef" class="h-full overflow-auto rounded border border-[var(--border)]" />
</template>
