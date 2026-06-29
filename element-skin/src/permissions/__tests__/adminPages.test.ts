import { describe, expect, it } from 'vitest'
import {
  adminPageForPath,
  canAccessAdminPath,
  firstAccessibleAdminPath,
  hasAnyAdminPagePermission,
} from '../adminPages'

describe('admin page permission access', () => {
  it('allows only notice admin pages for notice permissions exactly', () => {
    const permissions = ['notice.create.any']

    expect(hasAnyAdminPagePermission(permissions)).toBe(true)
    expect(firstAccessibleAdminPath(permissions)).toBe('/admin/notices')
    expect(canAccessAdminPath('/admin/notices', permissions)).toBe(true)
    expect(canAccessAdminPath('/admin/notices/123', permissions)).toBe(true)
    expect(canAccessAdminPath('/admin/settings', permissions)).toBe(false)
    expect(canAccessAdminPath('/admin/users', permissions)).toBe(false)
  })

  it('matches nested admin paths without matching similar prefixes exactly', () => {
    expect(adminPageForPath('/admin/homepage-media/42')?.path).toBe('/admin/homepage-media')
    expect(adminPageForPath('/admin/homepage-media-extra')).toBeNull()
    expect(adminPageForPath('/admin')).toBeNull()
  })

  it('denies users with no page-related permissions exactly', () => {
    const permissions = ['texture.read.owned', 'yggdrasil.join_server.owned']

    expect(hasAnyAdminPagePermission(permissions)).toBe(false)
    expect(firstAccessibleAdminPath(permissions)).toBeUndefined()
    expect(canAccessAdminPath('/admin/textures', permissions)).toBe(false)
  })

  it('uses the configured page order for the first accessible admin page exactly', () => {
    expect(firstAccessibleAdminPath(['site_settings.read.any', 'texture.delete.any'])).toBe(
      '/admin/textures',
    )
    expect(firstAccessibleAdminPath(['site_settings.read.any'])).toBe('/admin/settings')
  })
})
