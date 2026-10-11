import { normalizePageSize } from './pageSize'

export type ColorTheme = 'dark' | 'light'

const SITE_NAME_FALLBACK = '皮肤站'
const SITE_SUBTITLE_FALLBACK = '简洁、高效、现代的 Minecraft 皮肤 management 站'

const localStorageKeys = {
  siteName: 'site_name_cache',
  siteSubtitle: 'site_subtitle_cache',
  enableSkinLibrary: 'enable_skin_library_cache',
  theme: 'theme',
  easterEggDisabled: 'disableEasterEgg',
  pageSize: 'page_size_preference',
} as const

const activeLocalStorageKeys = new Set<string>(Object.values(localStorageKeys))

function storage(kind: 'local' | 'session'): Storage | null {
  if (typeof window === 'undefined') return null
  try {
    return kind === 'local' ? window.localStorage : window.sessionStorage
  } catch {
    return null
  }
}

function getString(kind: 'local' | 'session', key: string): string | null {
  try {
    return storage(kind)?.getItem(key) ?? null
  } catch {
    return null
  }
}

function setString(kind: 'local' | 'session', key: string, value: string): void {
  try {
    storage(kind)?.setItem(key, value)
  } catch {
    // Storage can be unavailable in private mode or full-quota situations.
  }
}

function remove(kind: 'local' | 'session', key: string): void {
  try {
    storage(kind)?.removeItem(key)
  } catch {
    // Same failure modes as setItem; callers treat storage as a best-effort cache.
  }
}

function cleanupUnusedLocalStorageKeys(): void {
  const local = storage('local')
  if (!local) return
  try {
    const keysToRemove: string[] = []
    for (let i = 0; i < local.length; i++) {
      const key = local.key(i)
      if (key && !activeLocalStorageKeys.has(key)) keysToRemove.push(key)
    }
    keysToRemove.forEach((key) => remove('local', key))
  } catch {
    // Best-effort cleanup only.
  }
}

export const appStorage = {
  cleanupUnusedKeys(): void {
    cleanupUnusedLocalStorageKeys()
  },

  siteSettings: {
    getSiteName(fallback = SITE_NAME_FALLBACK): string {
      return getString('local', localStorageKeys.siteName) || fallback
    },
    setSiteName(value: string): void {
      setString('local', localStorageKeys.siteName, value)
    },
    getSiteSubtitle(fallback = SITE_SUBTITLE_FALLBACK): string {
      return getString('local', localStorageKeys.siteSubtitle) || fallback
    },
    setSiteSubtitle(value: string): void {
      setString('local', localStorageKeys.siteSubtitle, value)
    },
    getEnableSkinLibrary(fallback = true): boolean {
      const value = getString('local', localStorageKeys.enableSkinLibrary)
      if (value === null) return fallback
      return value === 'true'
    },
    setEnableSkinLibrary(value: boolean): void {
      setString('local', localStorageKeys.enableSkinLibrary, String(value))
    },
  },

  theme: {
    get(): ColorTheme | null {
      const value = getString('local', localStorageKeys.theme)
      return value === 'dark' || value === 'light' ? value : null
    },
    set(value: ColorTheme): void {
      setString('local', localStorageKeys.theme, value)
    },
    hasUserPreference(): boolean {
      return this.get() !== null
    },
  },

  pageSize: {
    get(fallback = 20): number {
      const value = getString('local', localStorageKeys.pageSize)
      return normalizePageSize(value, fallback)
    },
    set(value: number): void {
      setString('local', localStorageKeys.pageSize, String(normalizePageSize(value)))
    },
  },

  easterEgg: {
    isDisabled(): boolean {
      return getString('local', localStorageKeys.easterEggDisabled) === '1'
    },
    setDisabled(disabled: boolean): void {
      if (disabled) {
        setString('local', localStorageKeys.easterEggDisabled, '1')
        return
      }
      remove('local', localStorageKeys.easterEggDisabled)
    },
  },
}
