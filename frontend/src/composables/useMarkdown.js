import { marked } from 'marked'
import DOMPurify from 'dompurify'

// Chat messages — including LLM output — are untrusted. Parse with marked then
// sanitise with DOMPurify. DOMPurify strips javascript: URIs, inline event
// handlers, and dangerous tags by default.
marked.setOptions({
  gfm: true,
  breaks: true, // single newlines become <br> — matches chat UX
})

// Post-process to add the md-* classes the chat CSS targets. Done via regex on
// the parsed HTML (rather than a custom Renderer) because the marked renderer
// API has changed repeatedly between major versions — string-rewriting the
// finished output is less fragile.
function addClasses(html) {
  return html
    .replace(/<pre>/g, '<pre class="md-code-block">')
    // Add md-inline-code to every <code>. It's harmless on code blocks (the
    // <pre class="md-code-block"> parent is what the CSS targets) and much
    // more robust than trying to negate "only outside <pre>" with regex.
    .replace(/<code>/g, '<code class="md-inline-code">')
    .replace(/<(ul|ol)>/g, '<$1 class="md-list">')
}

export function renderMarkdown(text) {
  if (!text) return ''
  return DOMPurify.sanitize(addClasses(marked.parse(text)))
}
