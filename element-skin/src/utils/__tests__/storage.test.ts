import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appStorage } from '../storage'

const AVATAR_CACHE_MAX_BYTES = 2 * 1024 * 1024

function avatarValue(size: number, label: string): string {
  return label + '.'.repeat(size - label.length)
}

function avatarManifest(): { maxBytes: number; entries: Array<{ hash: string; size: number }> } {
  return JSON.parse(window.localStorage.getItem('avatar_cache_lru') || '{}')
}

describe('appStorage', () => {
  beforeEach(() => {
    vi.useRealTimers()
    window.localStorage.clear()
    window.sessionStorage.clear()
  })

  it('returns exact site defaults and persists site settings', () => {
    expect(appStorage.siteSettings.getSiteName()).toBe('皮肤站')
    expect(appStorage.siteSettings.getSiteSubtitle()).toBe(
      '简洁、高效、现代的 Minecraft 皮肤 management 站',
    )
    expect(appStorage.siteSettings.getEnableSkinLibrary()).toBe(true)

    appStorage.siteSettings.setSiteName('Element Skin')
    appStorage.siteSettings.setSiteSubtitle('Minecraft skin service')
    appStorage.siteSettings.setEnableSkinLibrary(false)

    expect(appStorage.siteSettings.getSiteName()).toBe('Element Skin')
    expect(appStorage.siteSettings.getSiteSubtitle()).toBe('Minecraft skin service')
    expect(appStorage.siteSettings.getEnableSkinLibrary()).toBe(false)

    appStorage.siteSettings.setEnableSkinLibrary(true)
    expect(appStorage.siteSettings.getEnableSkinLibrary()).toBe(true)
  })

  it('persists only supported theme values and treats invalid values as absent', () => {
    expect(appStorage.theme.get()).toBeNull()
    expect(appStorage.theme.hasUserPreference()).toBe(false)

    appStorage.theme.set('dark')
    expect(appStorage.theme.get()).toBe('dark')
    expect(appStorage.theme.hasUserPreference()).toBe(true)

    appStorage.theme.set('light')
    expect(appStorage.theme.get()).toBe('light')

    window.localStorage.setItem('theme', 'sepia')
    expect(appStorage.theme.get()).toBeNull()
    expect(appStorage.theme.hasUserPreference()).toBe(false)
  })

  it('stores avatar cache by hash without mixing entries', () => {
    expect(appStorage.avatar.get('hash-a')).toBeNull()

    appStorage.avatar.set('hash-a', 'data:image/png;base64,AAA')
    appStorage.avatar.set('hash-b', 'data:image/png;base64,BBB')

    expect(appStorage.avatar.get('hash-a')).toBe('data:image/png;base64,AAA')
    expect(appStorage.avatar.get('hash-b')).toBe('data:image/png;base64,BBB')

    appStorage.avatar.remove('hash-a')
    expect(appStorage.avatar.get('hash-a')).toBeNull()
    expect(appStorage.avatar.get('hash-b')).toBe('data:image/png;base64,BBB')
  })

  it('evicts the least-recently-used avatar entries when the total cache exceeds the limit', () => {
    vi.useFakeTimers()
    const first = avatarValue(800 * 1024, 'first')
    const second = avatarValue(800 * 1024, 'second')
    const third = avatarValue(800 * 1024, 'third')

    vi.setSystemTime(1_000)
    appStorage.avatar.set('first', first)
    vi.setSystemTime(2_000)
    appStorage.avatar.set('second', second)
    vi.setSystemTime(3_000)
    expect(appStorage.avatar.get('first')).toBe(first)
    vi.setSystemTime(4_000)
    appStorage.avatar.set('third', third)

    expect(appStorage.avatar.get('first')).toBe(first)
    expect(appStorage.avatar.get('second')).toBeNull()
    expect(appStorage.avatar.get('third')).toBe(third)
    expect(avatarManifest().maxBytes).toBe(AVATAR_CACHE_MAX_BYTES)
    expect(avatarManifest().entries.map((entry) => entry.hash).sort()).toEqual(['first', 'third'])
  })

  it('rejects a single avatar entry larger than the total cache limit', () => {
    appStorage.avatar.set('too-large', avatarValue(AVATAR_CACHE_MAX_BYTES + 1, 'oversized'))

    expect(appStorage.avatar.get('too-large')).toBeNull()
    expect(window.localStorage.getItem('avatar_cache_too-large')).toBeNull()
    expect(avatarManifest().entries).toEqual([])
  })

  it('removes avatar entries from both storage and the LRU manifest', () => {
    appStorage.avatar.set('removable', 'data:image/png;base64,REMOVABLE')
    expect(avatarManifest().entries).toEqual([
      {
        hash: 'removable',
        size: 'data:image/png;base64,REMOVABLE'.length,
        accessedAt: expect.any(Number),
      },
    ])

    appStorage.avatar.remove('removable')

    expect(appStorage.avatar.get('removable')).toBeNull()
    expect(window.localStorage.getItem('avatar_cache_removable')).toBeNull()
    expect(avatarManifest().entries).toEqual([])
  })

  it('discovers legacy avatar cache entries and evicts them before recently-used entries', () => {
    vi.useFakeTimers()
    const legacy = avatarValue(900 * 1024, 'legacy')
    const current = avatarValue(900 * 1024, 'current')
    const incoming = avatarValue(900 * 1024, 'incoming')
    window.localStorage.setItem('avatar_cache_legacy', legacy)

    vi.setSystemTime(10_000)
    appStorage.avatar.set('current', current)
    vi.setSystemTime(20_000)
    appStorage.avatar.set('incoming', incoming)

    expect(appStorage.avatar.get('legacy')).toBeNull()
    expect(appStorage.avatar.get('current')).toBe(current)
    expect(appStorage.avatar.get('incoming')).toBe(incoming)
    expect(avatarManifest().entries.map((entry) => entry.hash).sort()).toEqual(['current', 'incoming'])
  })

  it('stores the easter egg disabled preference with the current key only', () => {
    expect(appStorage.easterEgg.isDisabled()).toBe(false)

    appStorage.easterEgg.setDisabled(true)
    expect(appStorage.easterEgg.isDisabled()).toBe(true)
    expect(window.localStorage.getItem('disableEasterEgg')).toBe('1')

    appStorage.easterEgg.setDisabled(false)
    expect(appStorage.easterEgg.isDisabled()).toBe(false)
    expect(window.localStorage.getItem('disableEasterEgg')).toBeNull()
  })

  it('falls back without throwing when browser storage is unavailable', () => {
    const originalLocalStorage = window.localStorage
    const throwingStorage = {
      getItem: () => {
        throw new Error('storage unavailable')
      },
      setItem: () => {
        throw new Error('storage unavailable')
      },
      removeItem: () => {
        throw new Error('storage unavailable')
      },
    } as unknown as Storage

    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: throwingStorage,
    })

    expect(appStorage.siteSettings.getSiteName('Fallback Name')).toBe('Fallback Name')
    expect(appStorage.siteSettings.getEnableSkinLibrary(false)).toBe(false)
    expect(appStorage.theme.get()).toBeNull()
    expect(appStorage.easterEgg.isDisabled()).toBe(false)
    expect(() => appStorage.siteSettings.setSiteName('Ignored')).not.toThrow()
    expect(() => appStorage.avatar.set('hash', 'data')).not.toThrow()
    expect(() => appStorage.avatar.remove('hash')).not.toThrow()

    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: originalLocalStorage,
    })
  })
})
