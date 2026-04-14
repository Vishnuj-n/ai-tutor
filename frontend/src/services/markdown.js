import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

export function renderMarkdown(input) {
  const source = typeof input === 'string' ? input : ''
  const html = md.render(source)
  return DOMPurify.sanitize(html)
}
