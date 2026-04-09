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
  // Search panel
  '.cm-panels': {
    backgroundColor: '#16213e',
    color: '#e2e8f0',
  },
  '.cm-panels.cm-panels-top': {
    borderBottom: '1px solid #334155',
  },
  '.cm-panels.cm-panels-bottom': {
    borderTop: '1px solid #334155',
  },
  '.cm-search': {
    fontSize: '13px',
  },
  '.cm-search label': {
    color: '#94a3b8',
  },
  '.cm-search input, .cm-search select': {
    backgroundColor: '#1a1a2e',
    color: '#e2e8f0',
    border: '1px solid #334155',
    borderRadius: '4px',
    padding: '2px 6px',
    fontSize: '13px',
    outline: 'none',
  },
  '.cm-search input:focus': {
    borderColor: '#3b82f6',
  },
  '.cm-search button': {
    backgroundColor: '#2a3a5c',
    color: '#e2e8f0',
    borderRadius: '4px',
    border: '1px solid #334155',
    padding: '2px 8px',
    cursor: 'pointer',
    fontSize: '13px',
  },
  '.cm-search button:hover': {
    backgroundColor: '#3b82f6',
  },
  '.cm-searchMatch': {
    backgroundColor: 'rgba(251, 191, 36, 0.3)',
    outline: '1px solid rgba(251, 191, 36, 0.5)',
  },
  '.cm-searchMatch.cm-searchMatch-selected': {
    backgroundColor: 'rgba(59, 130, 246, 0.4)',
  },
  '.cm-selectionMatch': {
    backgroundColor: 'rgba(59, 130, 246, 0.2)',
  },
  // Indent markers
  '.cm-indent-markers::before': {
    borderColor: '#334155',
  },
  '.cm-indent-markers.cm-indent-markers-active::before': {
    borderColor: '#475569',
  },
  // Autocomplete dropdown
  '.cm-tooltip-autocomplete': {
    backgroundColor: '#1e293b',
    border: '1px solid #334155',
  },
  '.cm-tooltip-autocomplete > ul': {
    fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
    fontSize: '13px',
  },
  '.cm-tooltip-autocomplete > ul > li': {
    padding: '2px 8px',
    color: '#e2e8f0',
  },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
    backgroundColor: '#3b82f6',
    color: '#ffffff',
  },
  '.cm-completionDetail': {
    color: '#64748b',
    fontStyle: 'italic',
    marginLeft: '8px',
  },
  '.cm-completionLabel': {
    color: '#60a5fa',
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
