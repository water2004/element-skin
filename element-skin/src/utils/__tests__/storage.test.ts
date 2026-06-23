import { beforeEach, describe, expect, it } from 'vitest'

import { appStorage } from '../storage'

describe('appStorage', () => {
  beforeEach(() => {
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
