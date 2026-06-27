import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({
  breaks: true,
  html: false,
  linkify: true,
})

md.disable(['table'])

const defaultLinkOpen =
  md.renderer.rules.link_open ||
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  if (!token) return ''
  token.attrSet('rel', 'noopener noreferrer')
  return defaultLinkOpen(tokens, idx, options, env, self)
}

md.renderer.rules.image = (tokens, idx) => {
  const token = tokens[idx]
  if (!token) return ''
  return `![${md.utils.escapeHtml(token.content)}](${md.utils.escapeHtml(token.attrGet('src') || '')})`
}

export function renderMarkdown(markdown: string): string {
  return DOMPurify.sanitize(md.render(markdown || ''))
}
