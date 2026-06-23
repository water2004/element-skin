export type ColorTheme = 'dark' | 'light'

const SITE_NAME_FALLBACK = '皮肤站'
const SITE_SUBTITLE_FALLBACK = '简洁、高效、现代的 Minecraft 皮肤 management 站'

const keys = {
  siteName: 'site_name_cache',
  siteSubtitle: 'site_subtitle_cache',
  enableSkinLibrary: 'enable_skin_library_cache',
  theme: 'theme',
  avatarPrefix: 'avatar_cache_',
  easterEggDisabled: 'disableEasterEgg',
} as const

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

function avatarKey(hash: string): string {
  return `${keys.avatarPrefix}${hash}`
}

export const appStorage = {
  siteSettings: {
    getSiteName(fallback = SITE_NAME_FALLBACK): string {
      return getString('local', keys.siteName) || fallback
    },
    setSiteName(value: string): void {
      setString('local', keys.siteName, value)
    },
    getSiteSubtitle(fallback = SITE_SUBTITLE_FALLBACK): string {
      return getString('local', keys.siteSubtitle) || fallback
    },
    setSiteSubtitle(value: string): void {
      setString('local', keys.siteSubtitle, value)
    },
    getEnableSkinLibrary(fallback = true): boolean {
      const value = getString('local', keys.enableSkinLibrary)
      if (value === null) return fallback
      return value === 'true'
    },
    setEnableSkinLibrary(value: boolean): void {
      setString('local', keys.enableSkinLibrary, String(value))
    },
  },

  theme: {
    get(): ColorTheme | null {
      const value = getString('local', keys.theme)
      return value === 'dark' || value === 'light' ? value : null
    },
    set(value: ColorTheme): void {
      setString('local', keys.theme, value)
    },
    hasUserPreference(): boolean {
      return this.get() !== null
    },
  },

  avatar: {
    get(hash: string): string | null {
      return getString('local', avatarKey(hash))
    },
    set(hash: string, dataUrl: string): void {
      setString('local', avatarKey(hash), dataUrl)
    },
    remove(hash: string): void {
      remove('local', avatarKey(hash))
    },
  },

  easterEgg: {
    isDisabled(): boolean {
      return getString('local', keys.easterEggDisabled) === '1'
    },
    setDisabled(disabled: boolean): void {
      if (disabled) {
        setString('local', keys.easterEggDisabled, '1')
        return
      }
      remove('local', keys.easterEggDisabled)
    },
  },
}
