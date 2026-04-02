/**
 * Lightweight markdown-to-HTML converter.
 * Handles: fenced code blocks, inline code, bold, italic, unordered/ordered lists, and line breaks.
 * No external dependencies.
 */
export function renderMarkdown(text) {
  if (!text) return ''

  // Escape HTML entities first
  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  // Fenced code blocks: ```lang\n...\n```
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
    return `<pre class="md-code-block"><code>${code.replace(/\n$/, '')}</code></pre>`
  })

  // Process line-by-line for lists and paragraphs
  const lines = html.split('\n')
  const output = []
  let inList = false
  let listType = null

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    // Skip lines inside code blocks (already handled above)
    if (line.includes('<pre class="md-code-block">') || line.includes('</pre>')) {
      if (inList) {
        output.push(listType === 'ul' ? '</ul>' : '</ol>')
        inList = false
        listType = null
      }
      output.push(line)
      continue
    }

    // Unordered list: - item or * item
    const ulMatch = line.match(/^(\s*)[-*]\s+(.+)/)
    if (ulMatch) {
      if (!inList || listType !== 'ul') {
        if (inList) output.push(listType === 'ul' ? '</ul>' : '</ol>')
        output.push('<ul class="md-list">')
        inList = true
        listType = 'ul'
      }
      output.push(`<li>${inlineFormat(ulMatch[2])}</li>`)
      continue
    }

    // Ordered list: 1. item
    const olMatch = line.match(/^(\s*)\d+\.\s+(.+)/)
    if (olMatch) {
      if (!inList || listType !== 'ol') {
        if (inList) output.push(listType === 'ul' ? '</ul>' : '</ol>')
        output.push('<ol class="md-list">')
        inList = true
        listType = 'ol'
      }
      output.push(`<li>${inlineFormat(olMatch[2])}</li>`)
      continue
    }

    // End list if we hit a non-list line
    if (inList) {
      output.push(listType === 'ul' ? '</ul>' : '</ol>')
      inList = false
      listType = null
    }

    // Empty line
    if (line.trim() === '') {
      output.push('')
      continue
    }

    // Regular text line
    output.push(inlineFormat(line))
  }

  if (inList) {
    output.push(listType === 'ul' ? '</ul>' : '</ol>')
  }

  // Collapse runs of 3+ newlines into a double newline (one blank line max)
  let result = output.join('\n')
  result = result.replace(/\n{3,}/g, '\n\n')
  return result
}

/** Apply inline formatting: bold, italic, inline code */
function inlineFormat(text) {
  // Inline code: `code`
  text = text.replace(/`([^`]+)`/g, '<code class="md-inline-code">$1</code>')
  // Bold: **text** or __text__
  text = text.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  text = text.replace(/__(.+?)__/g, '<strong>$1</strong>')
  // Italic: *text* or _text_ (but not inside words with underscores)
  text = text.replace(/(?<!\w)\*([^*]+)\*(?!\w)/g, '<em>$1</em>')
  text = text.replace(/(?<!\w)_([^_]+)_(?!\w)/g, '<em>$1</em>')
  return text
}
