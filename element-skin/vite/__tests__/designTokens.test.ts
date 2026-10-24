// @vitest-environment node
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { JSDOM } from 'jsdom'
import { expect, it } from 'vitest'
import { designTokenCases } from './fixtures/designTokenCases'

const root = fileURLToPath(new URL('../..', import.meta.url))
const read = (file: string) => fs.readFile(path.join(root, file), 'utf8')
const [appearance, element, dark, tokens, overrides] = await Promise.all([
  read('src/assets/styles/appearance.css'),
  read('node_modules/element-plus/dist/index.css'),
  read('node_modules/element-plus/theme-chalk/dark/css-vars.css'),
  read('src/assets/styles/tokens.css'),
  read('src/assets/styles/element-overrides.css'),
])

function createDocument(theme: string) {
  return new JSDOM(
    `<html class="${theme}"><head>${[appearance, element, dark, tokens, overrides]
      .map((css) => `<style>${css}</style>`)
      .join(
        '',
      )}</head><body><div class="el-input"></div><div class="el-textarea"></div></body></html>`,
  )
}

// CSSOM 保留自定义属性中的 var()；按实际层叠后的值解析，检查缺失引用与循环。
function resolveVariable(element: Element, name: string, seen = new Set<string>()): string {
  if (seen.has(name)) throw new Error(`Cyclic token: ${[...seen, name].join(' -> ')}`)
  const value = element.ownerDocument.defaultView!.getComputedStyle(element).getPropertyValue(name)
  if (!value.trim()) throw new Error(`Missing token: ${name}`)
  return value
    .trim()
    .replace(/var\((--[\w-]+)\)/g, (_, dependency: string) =>
      resolveVariable(element, dependency, new Set([...seen, name])),
    )
    .replace(/,\s*/g, ',')
}

it.each(designTokenCases)('maps actual Element Plus controls to the $theme theme', (testCase) => {
  const dom = createDocument(testCase.theme)
  try {
    const document = dom.window.document
    const html = document.documentElement
    expect(resolveVariable(html, '--el-bg-color-page')).toBe(testCase.page)
    expect(resolveVariable(html, '--el-bg-color')).toBe(testCase.surface)
    expect(resolveVariable(html, '--el-bg-color-overlay')).toBe(testCase.overlay)
    expect(resolveVariable(html, '--el-fill-color-blank')).toBe(testCase.surface)
    expect(resolveVariable(html, '--el-text-color-disabled')).toBe(testCase.disabledText)
    expect(resolveVariable(html, '--el-component-size-small')).toBe('24px')
    expect(resolveVariable(html, '--el-component-size')).toBe('32px')
    expect(resolveVariable(html, '--el-component-size-large')).toBe('40px')
    for (const selector of ['.el-input', '.el-textarea']) {
      const input = document.querySelector(selector)!
      expect(resolveVariable(input, '--el-input-bg-color')).toBe(testCase.surface)
      expect(resolveVariable(input, '--el-input-text-color')).toBe(testCase.text)
      expect(resolveVariable(input, '--el-input-placeholder-color')).toBe(testCase.secondaryText)
      expect(resolveVariable(input, '--el-input-border-color')).toBe(testCase.border)
      expect(resolveVariable(input, '--el-input-hover-border-color')).toBe(testCase.hoverBorder)
      expect(resolveVariable(input, '--el-input-focus-border-color')).toBe('#409eff')
    }
  } finally {
    dom.window.close()
  }
})

it.each(designTokenCases)(
  'resolves every application token without cycles in $theme',
  (testCase) => {
    const dom = createDocument(testCase.theme)
    try {
      const names = new Set(
        [...`${appearance}\n${tokens}`.matchAll(/(--[\w-]+)\s*:/g)].map((m) => m[1]!),
      )
      const unresolved: string[] = []
      for (const name of names) {
        const value = resolveVariable(dom.window.document.documentElement, name)
        if (value.includes('var(')) unresolved.push(`${name}: ${value}`)
      }
      expect(unresolved).toEqual([])
    } finally {
      dom.window.close()
    }
  },
)

it('updates inherited control variables when switching theme and changing the palette', () => {
  const dom = createDocument('light')
  try {
    const html = dom.window.document.documentElement
    const input = dom.window.document.querySelector('.el-input')!
    expect(resolveVariable(input, '--el-input-bg-color')).toBe('#ffffff')
    html.className = 'dark'
    expect(resolveVariable(input, '--el-input-bg-color')).toBe('#222222')
    expect(resolveVariable(html, '--el-bg-color-overlay')).toBe('#282828')
    html.style.setProperty('--el-color-primary', '#a34fff')
    expect(resolveVariable(input, '--el-input-focus-border-color')).toBe('#a34fff')
    expect(resolveVariable(html, '--color-selection-background')).toBe(
      'color-mix(in srgb,#a34fff 20%,transparent)',
    )
    html.className = 'light'
    expect(resolveVariable(input, '--el-input-bg-color')).toBe('#ffffff')
    expect(resolveVariable(html, '--color-selection-background')).toBe(
      'color-mix(in srgb,#a34fff 10%,transparent)',
    )
    expect(resolveVariable(input, '--el-input-focus-border-color')).toBe('#a34fff')
  } finally {
    dom.window.close()
  }
})
