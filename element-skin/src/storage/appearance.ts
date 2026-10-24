// 保持无导入依赖：同一模块也由 Vite 编译为 HTML head 中的同步启动脚本。
export type ColorTheme = 'dark' | 'light'

export const THEME_STORAGE_KEY = 'theme'

export const themeStorage = {
  get(): ColorTheme | null {
    try {
      const value = window.localStorage.getItem(THEME_STORAGE_KEY)
      return value === 'dark' || value === 'light' ? value : null
    } catch {
      return null
    }
  },
  set(value: ColorTheme): void {
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, value)
    } catch {
      // 存储不可用时仍可切换当前文档的主题。
    }
  },
  hasUserPreference(): boolean {
    return this.get() !== null
  },
}

export function resolveTheme(): ColorTheme {
  return (
    themeStorage.get() ??
    (window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
  )
}

export function applyTheme(theme: ColorTheme): void {
  const root = document.documentElement
  root.classList.toggle('dark', theme === 'dark')
  root.classList.toggle('light', theme === 'light')
}

export function applyPageAppearance(path: string): void {
  document.documentElement.classList.toggle('is-home-page', path === '/')
}

export function initializeAppearance(base: string): void {
  applyTheme(resolveTheme())
  const basePath = new URL(base, window.location.href).pathname.replace(/\/$/, '')
  const pathname = window.location.pathname
  const path =
    pathname === basePath
      ? '/'
      : pathname.startsWith(`${basePath}/`)
        ? pathname.slice(basePath.length)
        : pathname
  applyPageAppearance(path)
}
