import { hasAnyPermission } from './adminPages'

export interface SitePageAccess {
  path: string
  permissions: string[]
  requiredPermissions?: string[]
  exact?: boolean
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
    path: '/dashboard/oauth/apps/new',
    permissions: ['oauth_app.create.owned'],
    exact: true,
  },
  {
    path: '/dashboard/oauth/apps/:client_id/edit',
    permissions: ['oauth_app.update.owned', 'oauth_app.delete.owned'],
    requiredPermissions: ['oauth_app.read.owned'],
    exact: true,
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
    sitePageAccess
      .filter((page) => matchesSitePagePath(normalized, page))
      .sort((left, right) => pathSegmentCount(right.path) - pathSegmentCount(left.path))[0] ?? null
  )
}

export function canAccessSitePath(path: string, userPermissions: readonly string[]) {
  const page = sitePageForPath(path)
  return !!page && canAccessSitePage(page, userPermissions)
}

export function firstAccessibleSitePath(userPermissions: readonly string[]) {
  return sitePageAccess.find((page) => !page.exact && canAccessSitePage(page, userPermissions))
    ?.path
}

function canAccessSitePage(page: SitePageAccess, userPermissions: readonly string[]) {
  return (
    hasAnyPermission(userPermissions, page.permissions) &&
    (page.requiredPermissions ?? []).every((permission) => userPermissions.includes(permission))
  )
}

function matchesSitePagePath(path: string, page: SitePageAccess) {
  const pathSegments = splitPath(path)
  const pageSegments = splitPath(page.path)
  if (pathSegments.length < pageSegments.length) return false
  if (page.exact && pathSegments.length !== pageSegments.length) return false
  return pageSegments.every(
    (segment, index) => segment.startsWith(':') || segment === pathSegments[index],
  )
}

function pathSegmentCount(path: string) {
  return splitPath(path).length
}

function splitPath(path: string) {
  return path.split('/').filter(Boolean)
}
