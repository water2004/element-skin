import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { appStorage } from '../storage'
import {
  applyPageAppearance,
  initializeAppearance,
  resolveTheme,
  themeStorage,
} from '../appearance'
import { appearancePaths, themeCases } from './fixtures/appearanceCases'

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.className = ''
  window.history.replaceState(null, '', '/')
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  document.documentElement.className = ''
  window.history.replaceState(null, '', '/')
})

function systemTheme(dark: boolean) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => ({ matches: dark })),
  )
}

describe('document appearance bootstrap', () => {
  it.each(themeCases)('uses $label before application mounting', (testCase) => {
    if (testCase.saved) window.localStorage.setItem('theme', testCase.saved)
    systemTheme(testCase.systemDark)
    initializeAppearance('/')
    expect(resolveTheme()).toBe(testCase.expected)
    expect(document.documentElement.classList.contains('dark')).toBe(testCase.expected === 'dark')
    expect(document.documentElement.classList.contains('light')).toBe(testCase.expected === 'light')
    expect(window.localStorage.getItem('theme')).toBe(testCase.saved)
    expect(appStorage.theme).toBe(themeStorage)
  })

  it.each(appearancePaths)('selects the initial background for $base at $path', (testCase) => {
    systemTheme(false)
    window.history.replaceState(null, '', testCase.path)
    initializeAppearance(testCase.base)
    expect(document.documentElement.classList.contains('is-home-page')).toBe(testCase.home)
  })

  it('updates the page background on navigation without changing the selected theme', () => {
    systemTheme(false)
    initializeAppearance('/')
    expect(document.documentElement.className).toBe('light is-home-page')
    applyPageAppearance('/dashboard/home')
    expect(document.documentElement.className).toBe('light')
    applyPageAppearance('/')
    expect(document.documentElement.className).toBe('light is-home-page')
    expect(window.localStorage.getItem('theme')).toBeNull()
  })

  it('uses system preference when reading storage is blocked and still applies manual changes', () => {
    systemTheme(true)
    const get = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError')
    })
    initializeAppearance('/')
    expect(document.documentElement.className).toBe('dark is-home-page')
    expect(themeStorage.hasUserPreference()).toBe(false)
    get.mockRestore()
    const set = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('full', 'QuotaExceededError')
    })
    expect(() => themeStorage.set('light')).not.toThrow()
    expect(window.localStorage.getItem('theme')).toBeNull()
    set.mockRestore()
  })
})
