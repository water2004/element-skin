import { describe, expect, it } from 'vitest'
import { renderMarkdown } from '../markdown'

describe('renderMarkdown', () => {
  it('renders markdown and strips unsafe html exactly', () => {
    expect(renderMarkdown('Hello **OAuth** <script>alert(1)</script>\n\n[docs](/oauth/apps)')).toBe(
      '<p>Hello <strong>OAuth</strong> &lt;script&gt;alert(1)&lt;/script&gt;</p>\n<p><a href="/oauth/apps" rel="noopener noreferrer">docs</a></p>\n',
    )
  })

  it('does not render markdown images exactly', () => {
    expect(renderMarkdown('![avatar](/avatar.png)')).toBe('<p>![avatar](/avatar.png)</p>\n')
  })
})
