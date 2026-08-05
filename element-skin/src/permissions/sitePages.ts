import { hasAnyPermission } from './adminPages'

export interface SitePageAccess {
  path: string
  permissions: string[]
}

export const sitePageAccess: SitePageAccess[] = [
  {
    path: '/dashboard/home',
    permissions: ['account.read.self'],
  },
  {
    path: '/skin-library',
    permissions: ['texture.read.public', 'wardrobe_entry.add.owned', 'texture.apply.owned'],
  },
  {
    path: '/notifications',
    permissions: ['notice.read.owned', 'notice.dismiss.owned', 'notice.read.any'],
  },
  {
    path: '/dashboard/wardrobe',
    permissions: [
      'texture.read.owned',
      'texture.create.owned',
      'texture.update_metadata.owned',
      'texture.update_visibility.owned',
      'texture.delete.owned',
      'texture.apply.owned',
      'texture.clear.owned',
      'wardrobe.read.owned',
      'wardrobe_entry.read.owned',
      'wardrobe_entry.add.owned',
      'wardrobe_entry.update.owned',
      'wardrobe_entry.remove.owned',
      'wardrobe_entry.apply.owned',
    ],
  },
  {
    path: '/dashboard/roles',
    permissions: [
      'profile.read.owned',
      'profile.create.owned',
      'profile.update.owned',
      'profile.delete.owned',
      'profile.read.bound_profile',
      'profile.update.bound_profile',
      'texture.apply.bound_profile',
      'texture.clear.bound_profile',
      'official_profile.read.owned',
      'official_profile.create.owned',
      'official_profile.refresh.owned',
      'official_profile.delete.owned',
    ],
  },
  {
    path: '/dashboard/identities',
    permissions: [
      'external_identity.read.owned',
      'external_identity.create.owned',
      'external_identity.update.owned',
      'external_identity.delete.owned',
      'official_profile.read.owned',
      'official_profile.refresh.owned',
      'official_profile.delete.owned',
    ],
  },
  {
    path: '/dashboard/profile',
    permissions: [
      'account.read.self',
      'account.update.self',
      'account_password.update.self',
      'account.delete.self',
    ],
  },
  {
    path: '/dashboard/oauth',
    permissions: [
      'oauth_app.read.owned',
      'oauth_app.create.owned',
      'oauth_app.update.owned',
      'oauth_app.delete.owned',
      'oauth_grant.read.owned',
      'oauth_grant.revoke.owned',
    ],
  },
  {
    path: '/oauth/authorize',
    permissions: ['account.read.self'],
  },
  {
    path: '/oauth/device',
    permissions: ['oauth_grant.read.owned', 'account.read.self'],
  },
]

export const protectedSitePrefixes = ['/dashboard', '/skin-library', '/notifications', '/oauth']

export function isProtectedSitePath(path: string) {
  return protectedSitePrefixes.some((prefix) => path === prefix || path.startsWith(`${prefix}/`))
}

export function sitePageForPath(path: string) {
  const normalized = path.replace(/\/+$/, '') || '/'
  return (
    sitePageAccess.find(
      (page) => normalized === page.path || normalized.startsWith(`${page.path}/`),
    ) ?? null
  )
}

export function canAccessSitePath(path: string, userPermissions: readonly string[]) {
  const page = sitePageForPath(path)
  return !!page && hasAnyPermission(userPermissions, page.permissions)
}

export function firstAccessibleSitePath(userPermissions: readonly string[]) {
  return sitePageAccess.find((page) => hasAnyPermission(userPermissions, page.permissions))?.path
}
