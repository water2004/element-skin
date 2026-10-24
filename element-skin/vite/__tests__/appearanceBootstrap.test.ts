// @vitest-environment node
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import type { ResolvedConfig } from 'vite'
import { JSDOM } from 'jsdom'
import { expect, it, vi } from 'vitest'
import { appearanceBootstrap } from '../appearanceBootstrap'

const root = fileURLToPath(new URL('../..', import.meta.url))

it.each(['/', '/skin/'])(
  'inlines synchronous appearance before the app for base %s',
  async (base) => {
    const plugin = appearanceBootstrap()
    if (typeof plugin.configResolved !== 'function') throw new Error('Expected config hook')
    plugin.configResolved({ root, base } as ResolvedConfig)
    const hook = plugin.transformIndexHtml
    if (!hook || typeof hook !== 'object' || !('handler' in hook))
      throw new Error('Expected HTML hook')
    const html = await fs.readFile(path.join(root, 'index.html'), 'utf8')
    const transformed = await hook.handler(html, {
      path: '/',
      filename: path.join(root, 'index.html'),
    })
    if (typeof transformed !== 'string') throw new Error('Expected transformed HTML')
    expect(transformed.indexOf('<meta charset=')).toBeLessThan(1024)
    expect(transformed.indexOf('<style id="initial-appearance">')).toBeLessThan(
      transformed.indexOf('<script>'),
    )
    expect(transformed.indexOf('<script>')).toBeLessThan(transformed.indexOf('type="module"'))
    expect(transformed).not.toContain('<!-- appearance-bootstrap -->')

    const dom = new JSDOM(transformed, {
      url: `http://localhost${base}dashboard/home`,
      runScripts: 'outside-only',
    })
    try {
      const { window } = dom
      const { document } = window
      const script = document.querySelector('script:not([type])')!
      expect(script.hasAttribute('src')).toBe(false)
      expect(script.hasAttribute('defer')).toBe(false)
      expect(script.hasAttribute('async')).toBe(false)
      window.localStorage.setItem('theme', 'light')
      Object.defineProperty(window, 'matchMedia', { value: vi.fn(() => ({ matches: true })) })
      window.eval(script.textContent!)
      expect(document.documentElement.className).toBe('light')
      const rules = Array.from(document.styleSheets[0]!.cssRules) as CSSStyleRule[]
      const rootStyle = rules.find((rule) => rule.selectorText === ':root')!.style
      expect(rootStyle.getPropertyValue('--vt-c-white')).toBe('#ffffff')
      expect(rootStyle.getPropertyValue('--color-background')).toBe('var(--vt-c-white)')
      expect(rootStyle.getPropertyValue('color-scheme')).toBe('light')
      expect(rules.find((rule) => rule.selectorText === 'body')!.style.background).toBe(
        'var(--color-background)',
      )
      window.history.replaceState(null, '', base)
      window.eval(script.textContent!)
      expect(document.documentElement.className).toBe('light is-home-page')
      expect(rootStyle.getPropertyValue('--color-home-background')).toBe('#1a1a1a')
      expect(
        rules.find((rule) => rule.selectorText.includes('html.is-home-page body'))!.style
          .background,
      ).toBe('var(--color-home-background)')
    } finally {
      dom.window.close()
    }
  },
)
