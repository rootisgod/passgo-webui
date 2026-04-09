<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { yaml } from '@codemirror/lang-yaml'
import { linter, lintGutter } from '@codemirror/lint'
import { bracketMatching, foldGutter, indentOnInput } from '@codemirror/language'
import { closeBrackets, autocompletion, completionKeymap } from '@codemirror/autocomplete'
import { search, searchKeymap, highlightSelectionMatches } from '@codemirror/search'
import { indentationMarkers } from '@replit/codemirror-indentation-markers'
import YAML from 'js-yaml'
import { darkTheme, darkHighlightStyle } from './editorTheme.js'

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

// Known cloud-init top-level keys and their expected types
const CLOUD_INIT_KEYS = {
  // Package management
  packages: 'list', package_update: 'boolean', package_upgrade: 'boolean',
  package_reboot_if_required: 'boolean', apt: 'object',
  // Commands
  runcmd: 'list', bootcmd: 'list',
  // Files
  write_files: 'list',
  // Users & groups
  users: 'list', groups: 'list',
  // SSH
  ssh_authorized_keys: 'list', ssh_keys: 'object', ssh_pwauth: 'boolean',
  disable_root: 'boolean',
  // System
  hostname: 'string', fqdn: 'string', manage_etc_hosts: 'any',
  timezone: 'string', locale: 'string', ntp: 'object',
  // Disk
  disk_setup: 'object', fs_setup: 'list', mounts: 'list',
  swap: 'object', growpart: 'object',
  // Network
  network: 'object',
  // Power
  power_state: 'object',
  // Final message / phone home
  final_message: 'string', phone_home: 'object',
  // Misc
  chpasswd: 'object', snap: 'object', ca_certs: 'object',
  resolv_conf: 'object', keyboard: 'object', locale_configfile: 'string',
  manage_resolv_conf: 'boolean', preserve_hostname: 'boolean',
  apt_sources: 'list', yum_repos: 'object', zypper: 'object',
  chef: 'object', puppet: 'object', salt_minion: 'object', mcollective: 'object',
  byobu_by_default: 'string', output: 'object', random_seed: 'object',
  lxd: 'object', snap_commands: 'list',
}

function typeCheck(value, expected) {
  if (expected === 'any') return true
  if (expected === 'list') return Array.isArray(value)
  if (expected === 'boolean') return typeof value === 'boolean'
  if (expected === 'string') return typeof value === 'string'
  if (expected === 'object') return typeof value === 'object' && value !== null && !Array.isArray(value)
  return true
}

function typeName(expected) {
  if (expected === 'list') return 'a list'
  if (expected === 'boolean') return 'true or false'
  if (expected === 'string') return 'a string'
  if (expected === 'object') return 'a mapping'
  return expected
}

// Find the line number of a top-level key in the document
function findKeyLine(doc, key) {
  const pattern = new RegExp(`^${key}\\s*:`)
  for (let i = 1; i <= doc.lines; i++) {
    if (pattern.test(doc.line(i).text)) return i
  }
  return 1
}

function yamlLinter() {
  return linter((editorView) => {
    if (destroyed) return []
    const content = editorView.state.doc.toString()
    const doc = editorView.state.doc
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
    let parsed = null
    try {
      parsed = YAML.load(content)
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

    // Validate known keys and types if YAML parsed as an object
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      for (const key of Object.keys(parsed)) {
        const lineNum = findKeyLine(doc, key)
        const line = doc.line(lineNum)

        if (!(key in CLOUD_INIT_KEYS)) {
          diagnostics.push({
            from: line.from,
            to: line.to,
            severity: 'warning',
            message: `Unknown cloud-init key: "${key}"`,
          })
        } else {
          const expected = CLOUD_INIT_KEYS[key]
          if (parsed[key] !== null && !typeCheck(parsed[key], expected)) {
            diagnostics.push({
              from: line.from,
              to: line.to,
              severity: 'warning',
              message: `"${key}" should be ${typeName(expected)}`,
            })
          }
        }
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

function cloudInitCompletions(context) {
  const line = context.state.doc.lineAt(context.pos)
  const textBefore = context.state.sliceDoc(line.from, context.pos)
  const match = textBefore.match(/^(\w*)$/)
  if (!match) return null

  return {
    from: context.pos - match[1].length,
    options: Object.entries(CLOUD_INIT_KEYS).map(([key, type]) => ({
      label: key,
      detail: type,
      type: 'keyword',
      apply: `${key}: `,
    })),
    validFor: /^\w*$/,
  }
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
    autocompletion({ override: [cloudInitCompletions], activateOnTyping: true }),
    yaml(),
    darkTheme,
    darkHighlightStyle,
    lintGutter(),
    yamlLinter(),
    wrapCompartment.of(props.wordWrap ? EditorView.lineWrapping : []),
    search(),
    highlightSelectionMatches(),
    keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap, ...completionKeymap]),
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
  <div
    class="overflow-hidden"
    :class="fullscreen ? 'fixed inset-0 z-30 flex flex-col bg-[#1a1a2e]' : 'h-full flex flex-col rounded border border-[var(--border)]'"
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
