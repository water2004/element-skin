export type ColorTheme = 'dark' | 'light'

const SITE_NAME_FALLBACK = '皮肤站'
const SITE_SUBTITLE_FALLBACK = '简洁、高效、现代的 Minecraft 皮肤 management 站'
const AVATAR_CACHE_MAX_BYTES = 2 * 1024 * 1024

const keys = {
  siteName: 'site_name_cache',
  siteSubtitle: 'site_subtitle_cache',
  enableSkinLibrary: 'enable_skin_library_cache',
  theme: 'theme',
  avatarPrefix: 'avatar_cache_',
  avatarManifest: 'avatar_cache_lru',
  easterEggDisabled: 'disableEasterEgg',
} as const

interface AvatarCacheEntry {
  hash: string
  size: number
  accessedAt: number
}

interface AvatarCacheManifest {
  version: 1
  maxBytes: number
  entries: AvatarCacheEntry[]
}

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

function now(): number {
  return Date.now()
}

function avatarKey(hash: string): string {
  return `${keys.avatarPrefix}${hash}`
}

function byteSize(value: string): number {
  return value.length
}

function emptyAvatarManifest(): AvatarCacheManifest {
  return { version: 1, maxBytes: AVATAR_CACHE_MAX_BYTES, entries: [] }
}

function discoverAvatarEntries(knownEntries: AvatarCacheEntry[]): AvatarCacheEntry[] {
  const local = storage('local')
  if (!local) return []
  const knownHashes = new Set(knownEntries.map((entry) => entry.hash))
  const discovered: AvatarCacheEntry[] = []

  try {
    for (let i = 0; i < local.length; i++) {
      const key = local.key(i)
      if (!key || key === keys.avatarManifest || !key.startsWith(keys.avatarPrefix)) continue
      const hash = key.slice(keys.avatarPrefix.length)
      if (!hash || knownHashes.has(hash)) continue
      const value = getString('local', key)
      if (value === null) continue
      discovered.push({ hash, size: byteSize(value), accessedAt: 0 })
    }
  } catch {
    return []
  }

  return discovered
}

function readAvatarManifest(): AvatarCacheManifest {
  const raw = getString('local', keys.avatarManifest)
  if (!raw) {
    const empty = emptyAvatarManifest()
    return { ...empty, entries: discoverAvatarEntries(empty.entries) }
  }
  try {
    const parsed = JSON.parse(raw) as Partial<AvatarCacheManifest>
    if (!Array.isArray(parsed.entries)) {
      const empty = emptyAvatarManifest()
      return { ...empty, entries: discoverAvatarEntries(empty.entries) }
    }
    const manifest: AvatarCacheManifest = {
      version: 1,
      maxBytes: AVATAR_CACHE_MAX_BYTES,
      entries: parsed.entries
        .filter((entry): entry is AvatarCacheEntry => {
          return (
            typeof entry?.hash === 'string' &&
            typeof entry.size === 'number' &&
            Number.isFinite(entry.size) &&
            entry.size >= 0 &&
            typeof entry.accessedAt === 'number' &&
            Number.isFinite(entry.accessedAt)
          )
        })
        .map((entry) => ({
          hash: entry.hash,
          size: entry.size,
          accessedAt: entry.accessedAt,
        })),
    }
    return { ...manifest, entries: [...manifest.entries, ...discoverAvatarEntries(manifest.entries)] }
  } catch {
    const empty = emptyAvatarManifest()
    return { ...empty, entries: discoverAvatarEntries(empty.entries) }
  }
}

function writeAvatarManifest(manifest: AvatarCacheManifest): void {
  setString('local', keys.avatarManifest, JSON.stringify(manifest))
}

function avatarCacheTotal(entries: AvatarCacheEntry[]): number {
  return entries.reduce((total, entry) => total + entry.size, 0)
}

function upsertAvatarEntry(
  entries: AvatarCacheEntry[],
  hash: string,
  size: number,
  accessedAt = now(),
): AvatarCacheEntry[] {
  return [
    ...entries.filter((entry) => entry.hash !== hash),
    {
      hash,
      size,
      accessedAt,
    },
  ]
}

function pruneAvatarCache(manifest: AvatarCacheManifest, protectedHash?: string): AvatarCacheManifest {
  const existingEntries = manifest.entries.filter((entry) => {
    if (entry.hash === protectedHash) return true
    if (getString('local', avatarKey(entry.hash)) !== null) return true
    return false
  })
  const evictable = [...existingEntries]
    .filter((entry) => entry.hash !== protectedHash)
    .sort((a, b) => a.accessedAt - b.accessedAt)
  const kept = [...existingEntries]
  let total = avatarCacheTotal(kept)

  for (const entry of evictable) {
    if (total <= AVATAR_CACHE_MAX_BYTES) break
    remove('local', avatarKey(entry.hash))
    const idx = kept.findIndex((candidate) => candidate.hash === entry.hash)
    if (idx >= 0) kept.splice(idx, 1)
    total -= entry.size
  }

  return { version: 1, maxBytes: AVATAR_CACHE_MAX_BYTES, entries: kept }
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
      const value = getString('local', avatarKey(hash))
      if (value === null) return null

      const manifest = readAvatarManifest()
      const touched = {
        ...manifest,
        entries: upsertAvatarEntry(manifest.entries, hash, byteSize(value)),
      }
      writeAvatarManifest(pruneAvatarCache(touched, hash))
      return value
    },
    set(hash: string, dataUrl: string): void {
      const size = byteSize(dataUrl)
      if (size > AVATAR_CACHE_MAX_BYTES) {
        remove('local', avatarKey(hash))
        const manifest = readAvatarManifest()
        writeAvatarManifest({
          ...manifest,
          entries: manifest.entries.filter((entry) => entry.hash !== hash),
        })
        return
      }

      setString('local', avatarKey(hash), dataUrl)
      if (getString('local', avatarKey(hash)) !== dataUrl) return

      const manifest = readAvatarManifest()
      const updated = {
        ...manifest,
        entries: upsertAvatarEntry(manifest.entries, hash, size),
      }
      writeAvatarManifest(pruneAvatarCache(updated, hash))
    },
    remove(hash: string): void {
      remove('local', avatarKey(hash))
      const manifest = readAvatarManifest()
      writeAvatarManifest({
        ...manifest,
        entries: manifest.entries.filter((entry) => entry.hash !== hash),
      })
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
