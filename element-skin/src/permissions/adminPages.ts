export interface AdminPageAccess {
  path: string
  permissions: string[]
  requiredPermissions?: string[]
  exact?: boolean
}

export const adminPageAccess: AdminPageAccess[] = [
  {
    path: '/admin/users',
    permissions: [
      'user.read.any',
      'user.update.any',
      'account.read.any',
      'account.update.any',
      'account.delete.any',
      'account.ban.any',
      'account.unban.any',
      'permission.read.any',
      'permission.grant.any',
      'permission.revoke.any',
      'permission_protected.manage.any',
      'profile.read.any',
    ],
  },
  {
    path: '/admin/roles',
    permissions: ['profile.read.any', 'profile.update.any', 'profile.delete.any'],
  },
  {
    path: '/admin/textures',
    permissions: [
      'texture.read.any',
      'texture.update_metadata.any',
      'texture.update_visibility.any',
      'texture.delete.any',
    ],
  },
  {
    path: '/admin/invites',
    permissions: ['invite.read.any', 'invite.create.any', 'invite.delete.any'],
  },
  {
    path: '/admin/settings',
    permissions: ['site_settings.read.any', 'site_settings.update.any'],
  },
  {
    path: '/admin/email',
    permissions: ['site_settings.read.any', 'site_settings.update.any'],
  },
  {
    path: '/admin/notices',
    permissions: ['notice.read.any', 'notice.create.any', 'notice.update.any', 'notice.delete.any'],
  },
  {
    path: '/admin/oauth-apps',
    permissions: [
      'oauth_app.read.any',
      'oauth_app.update.any',
      'oauth_app.delete.any',
      'oauth_grant.read.any',
      'oauth_grant.revoke.any',
    ],
  },
  {
    path: '/admin/identity-providers',
    permissions: ['identity_provider.read.any'],
  },
  {
    path: '/admin/identity-providers/new',
    permissions: ['identity_provider.create.any'],
    exact: true,
  },
  {
    path: '/admin/identity-providers/:provider_id/edit',
    permissions: ['identity_provider.update.any'],
    requiredPermissions: ['identity_provider.read.any'],
    exact: true,
  },
  {
    path: '/admin/mojang',
    permissions: [
      'site_settings.read.any',
      'site_settings.update.any',
      'official_whitelist.read.any',
      'official_whitelist.add.any',
      'official_whitelist.remove.any',
    ],
  },
  {
    path: '/admin/homepage-media',
    permissions: [
      'homepage_media.read.any',
      'homepage_media.create.any',
      'homepage_media.update.any',
      'homepage_media.delete.any',
    ],
  },
  {
    path: '/admin/easter-eggs',
    permissions: ['site_settings.read.any', 'site_settings.update.any'],
  },
]

export const adminPagePermissions = Array.from(
  new Set(adminPageAccess.flatMap((page) => page.permissions)),
)

export function hasAnyPermission(
  userPermissions: readonly string[],
  requiredPermissions: readonly string[],
) {
  return requiredPermissions.some((permission) => userPermissions.includes(permission))
}

export function hasAnyAdminPagePermission(userPermissions: readonly string[]) {
  return hasAnyPermission(userPermissions, adminPagePermissions)
}

export function adminPageForPath(path: string) {
  const normalized = path.replace(/\/+$/, '') || '/'
  return (
    adminPageAccess
      .filter((page) => matchesAdminPagePath(normalized, page))
      .sort((left, right) => pathSegmentCount(right.path) - pathSegmentCount(left.path))[0] ?? null
  )
}

export function canAccessAdminPath(path: string, userPermissions: readonly string[]) {
  const page = adminPageForPath(path)
  return (
    !!page &&
    hasAnyPermission(userPermissions, page.permissions) &&
    (page.requiredPermissions ?? []).every((permission) => userPermissions.includes(permission))
  )
}

export function firstAccessibleAdminPath(userPermissions: readonly string[]) {
  return adminPageAccess.find(
    (page) =>
      !page.path.includes(':') &&
      hasAnyPermission(userPermissions, page.permissions) &&
      (page.requiredPermissions ?? []).every((permission) => userPermissions.includes(permission)),
  )?.path
}

function matchesAdminPagePath(path: string, page: AdminPageAccess) {
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
