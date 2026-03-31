import { EditorView } from '@codemirror/view'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { tags } from '@lezer/highlight'

export const darkTheme = EditorView.theme({
  '&': {
    backgroundColor: '#1a1a2e',
    color: '#e2e8f0',
    fontSize: '13px',
    height: '100%',
  },
  '.cm-content': {
    caretColor: '#3b82f6',
    fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
  },
  '.cm-gutters': {
    backgroundColor: '#16213e',
    color: '#64748b',
    borderRight: '1px solid #334155',
  },
  '.cm-activeLineGutter': {
    backgroundColor: '#2a3a5c',
  },
  '&.cm-focused .cm-activeLine': {
    backgroundColor: 'rgba(42, 58, 92, 0.3)',
  },
  '&.cm-focused .cm-cursor': {
    borderLeftColor: '#3b82f6',
  },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
    backgroundColor: 'rgba(59, 130, 246, 0.25)',
  },
  '.cm-line': {
    padding: '0 4px',
  },
  '.cm-tooltip': {
    backgroundColor: '#1e293b',
    border: '1px solid #334155',
    color: '#e2e8f0',
  },
  '.cm-tooltip-lint': {
    backgroundColor: '#1e293b',
  },
  '.cm-diagnostic-error': {
    borderLeft: '3px solid #ef4444',
  },
  '.cm-diagnostic-warning': {
    borderLeft: '3px solid #eab308',
  },
  '.cm-lintRange-error': {
    backgroundImage: 'none',
    textDecoration: 'underline wavy #ef4444',
  },
  '.cm-lintRange-warning': {
    backgroundImage: 'none',
    textDecoration: 'underline wavy #eab308',
  },
}, { dark: true })

export const darkHighlightStyle = syntaxHighlighting(HighlightStyle.define([
  { tag: tags.comment, color: '#64748b', fontStyle: 'italic' },
  { tag: tags.string, color: '#22c55e' },
  { tag: tags.number, color: '#f59e0b' },
  { tag: tags.bool, color: '#f59e0b' },
  { tag: tags.null, color: '#f59e0b' },
  { tag: tags.keyword, color: '#a78bfa' },
  { tag: tags.propertyName, color: '#60a5fa' },
  { tag: tags.definition(tags.propertyName), color: '#60a5fa' },
  { tag: tags.meta, color: '#94a3b8' },
  { tag: tags.punctuation, color: '#94a3b8' },
]))
