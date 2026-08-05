import { describe, expect, it } from 'vitest'
import {
  canAccessSitePath,
  firstAccessibleSitePath,
  isProtectedSitePath,
  sitePageForPath,
} from '../sitePages'

describe('site page permission access', () => {
  it('allows only wardrobe pages for texture ownership permissions exactly', () => {
    const permissions = ['texture.read.owned']

    expect(firstAccessibleSitePath(permissions)).toBe('/dashboard/wardrobe')
    expect(canAccessSitePath('/dashboard/wardrobe', permissions)).toBe(true)
    expect(canAccessSitePath('/dashboard/wardrobe/detail', permissions)).toBe(true)
    expect(canAccessSitePath('/dashboard/roles', permissions)).toBe(false)
    expect(canAccessSitePath('/notifications', permissions)).toBe(false)
  })

  it('routes account-only users to dashboard home before profile exactly', () => {
    const permissions = ['account.read.self', 'account.update.self']

    expect(firstAccessibleSitePath(permissions)).toBe('/dashboard/home')
    expect(canAccessSitePath('/dashboard/home', permissions)).toBe(true)
    expect(canAccessSitePath('/dashboard/profile', permissions)).toBe(true)
    expect(canAccessSitePath('/reset-email', permissions)).toBe(false)
  })

  it('matches nested paths without matching similar prefixes exactly', () => {
    expect(sitePageForPath('/notifications/notice-1')?.path).toBe('/notifications')
    expect(sitePageForPath('/dashboard/roles/import')?.path).toBe('/dashboard/roles')
    expect(sitePageForPath('/dashboardish')).toBeNull()
  })

  it('identifies protected site paths exactly', () => {
    expect(isProtectedSitePath('/dashboard')).toBe(true)
    expect(isProtectedSitePath('/dashboard/home')).toBe(true)
    expect(isProtectedSitePath('/skin-library')).toBe(true)
    expect(isProtectedSitePath('/notifications/notice-1')).toBe(true)
    expect(isProtectedSitePath('/reset-email')).toBe(false)
    expect(isProtectedSitePath('/')).toBe(false)
    expect(isProtectedSitePath('/admin/users')).toBe(false)
  })

  it('exposes identity and official binding pages through fine-grained permissions', () => {
    expect(canAccessSitePath('/dashboard/identities', ['external_identity.read.owned'])).toBe(true)
    expect(canAccessSitePath('/dashboard/identities', ['official_profile.read.owned'])).toBe(true)
    expect(canAccessSitePath('/dashboard/roles', ['official_profile.refresh.owned'])).toBe(true)
    expect(canAccessSitePath('/dashboard/identities', ['profile.read.owned'])).toBe(false)
  })
})
