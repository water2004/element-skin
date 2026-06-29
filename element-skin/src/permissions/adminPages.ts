export interface AdminPageAccess {
  path: string
  permissions: string[]
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
    adminPageAccess.find(
      (page) => normalized === page.path || normalized.startsWith(`${page.path}/`),
    ) ?? null
  )
}

export function canAccessAdminPath(path: string, userPermissions: readonly string[]) {
  const page = adminPageForPath(path)
  return !!page && hasAnyPermission(userPermissions, page.permissions)
}

export function firstAccessibleAdminPath(userPermissions: readonly string[]) {
  return adminPageAccess.find((page) => hasAnyPermission(userPermissions, page.permissions))?.path
}
