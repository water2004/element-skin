import { createApp, h } from 'vue'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { appStorage } from '@/storage'
import { initializeAppearance } from '@/storage/appearance'
import { useTheme } from '../useTheme'

let cleanup: (() => void) | undefined
let preferenceChange: (event: { matches: boolean }) => void
const addListener = vi.fn((_name, callback) => {
  preferenceChange = callback
})
const removeListener = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
  window.localStorage.clear()
  document.documentElement.className = ''
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => ({
      matches: true,
      addEventListener: addListener,
      removeEventListener: removeListener,
    })),
  )
})
afterEach(() => {
  cleanup?.()
  cleanup = undefined
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  document.documentElement.className = ''
})

function mountTheme() {
  let theme!: ReturnType<typeof useTheme>
  const app = createApp({
    setup() {
      theme = useTheme()
      return () => h('div')
    },
  })
  app.mount(document.createElement('div'))
  cleanup = () => app.unmount()
  return theme
}

it('keeps explicit light mode from bootstrap through mounting and system dark changes', () => {
  appStorage.theme.set('light')
  initializeAppearance('/')
  const theme = mountTheme()
  expect(theme.isDark.value).toBe(false)
  theme.initTheme()
  preferenceChange({ matches: true })
  expect(theme.isDark.value).toBe(false)
  expect(document.documentElement.classList.contains('light')).toBe(true)
  expect(document.documentElement.classList.contains('dark')).toBe(false)
  expect(appStorage.theme.get()).toBe('light')
})

it('follows the system until an explicit toggle and removes the exact watcher on unmount', () => {
  const theme = mountTheme()
  theme.initTheme()
  theme.initTheme()
  expect(addListener).toHaveBeenCalledTimes(1)
  expect(theme.isDark.value).toBe(true)
  preferenceChange({ matches: false })
  expect(theme.isDark.value).toBe(false)
  expect(appStorage.theme.get()).toBeNull()
  theme.toggleTheme()
  expect(theme.isDark.value).toBe(true)
  expect(appStorage.theme.get()).toBe('dark')
  preferenceChange({ matches: false })
  expect(theme.isDark.value).toBe(true)
  expect(document.documentElement.className).toBe('dark')
  cleanup!()
  cleanup = undefined
  expect(removeListener).toHaveBeenCalledExactlyOnceWith('change', preferenceChange)
})

it('applies both theme classes correctly when persisting a manual toggle fails', () => {
  appStorage.theme.set('light')
  const theme = mountTheme()
  theme.initTheme()
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new DOMException('full', 'QuotaExceededError')
  })
  theme.toggleTheme()
  expect(theme.isDark.value).toBe(true)
  expect(document.documentElement.className).toBe('dark')
  expect(appStorage.theme.get()).toBe('light')
})
