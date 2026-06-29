import type { Component } from 'vue'
import {
  Back,
  Bell,
  Box,
  Link,
  MagicStick,
  Message,
  Odometer,
  Picture,
  Setting,
  Tools,
  User,
} from '@element-plus/icons-vue'
import { canAccessAdminPath, hasAnyAdminPagePermission } from '@/permissions/adminPages'
import { canAccessSitePath } from '@/permissions/sitePages'

export interface NavLink {
  type?: 'item' | 'group'
  path?: string
  index?: string
  trigger?: 'hover' | 'click'
  title?: string
  icon?: Component
  children?: NavLink[]
}

export interface DrawerLink {
  isDivider?: boolean
  path?: string
  title?: string
  icon?: Component
}

const dashboardLinks: NavLink[] = [
  { path: '/dashboard/home', title: '仪表盘', icon: Odometer },
  { path: '/notifications', title: '通知中心', icon: Bell },
  { path: '/dashboard/wardrobe', title: '我的衣柜', icon: Box },
  { path: '/dashboard/roles', title: '角色管理', icon: User },
  { path: '/dashboard/profile', title: '个人资料', icon: Setting },
]

const adminNavLinks: NavLink[] = [
  { path: '/dashboard', title: '返回面板', icon: Back },
  { path: '/admin/users', title: '用户管理', icon: User },
  { path: '/admin/roles', title: '角色管理', icon: User },
  { path: '/admin/textures', title: '材质管理', icon: Box },
  { path: '/admin/invites', title: '邀请码管理', icon: Tools },
  { path: '/admin/settings', title: '站点设置', icon: Setting },
  { path: '/admin/email', title: '邮件服务', icon: Message },
  { path: '/admin/notices', title: '通知公告', icon: Bell },
  { path: '/admin/mojang', title: 'Fallback 服务', icon: Link },
  { path: '/admin/homepage-media', title: '首页图片', icon: Picture },
  { path: '/admin/easter-eggs', title: '彩蛋列表', icon: MagicStick },
]

export function canAccessAdmin(userPermissions: string[]) {
  return hasAnyAdminPagePermission(userPermissions)
}

export function buildDefaultOpeneds(path: string) {
  const opened: string[] = []
  if (['/admin/users', '/admin/roles', '/admin/textures'].some((item) => path.startsWith(item))) {
    opened.push('admin-content-group')
  }
  if (
    [
      '/admin/email',
      '/admin/notices',
      '/admin/mojang',
      '/admin/homepage-media',
      '/admin/easter-eggs',
    ].some((item) => path.startsWith(item))
  ) {
    opened.push('admin-config-group')
  }
  return opened
}

export function buildNavLinks(input: {
  path: string
  isLogged: boolean
  enableSkinLibrary: boolean
  userPermissions: string[]
}) {
  if (input.path.startsWith('/admin')) return buildAdminNavItems(input.userPermissions)
  if (!input.isLogged) return []

  const links: NavLink[] = []
  if (input.enableSkinLibrary && canAccessSitePath('/skin-library', input.userPermissions)) {
    links.push({ path: '/skin-library', title: '皮肤库', icon: Picture })
  }
  links.push(...dashboardLinks.filter((item) => canAccessSiteLink(item, input.userPermissions)))
  if (canAccessAdmin(input.userPermissions))
    links.push({ path: '/admin', title: '管理面板', icon: Tools })
  return links
}

export function buildDrawerLinks(input: {
  isLogged: boolean
  enableSkinLibrary: boolean
  userPermissions: string[]
}) {
  if (!input.isLogged) return []

  const links: DrawerLink[] = []
  if (input.enableSkinLibrary && canAccessSitePath('/skin-library', input.userPermissions)) {
    links.push({ path: '/skin-library', title: '皮肤库', icon: Picture })
  }
  links.push({ isDivider: true })
  links.push(...dashboardLinks.filter((item) => canAccessSiteLink(item, input.userPermissions)))
  if (canAccessAdmin(input.userPermissions)) {
    links.push({ isDivider: true })
    links.push(...filterAdminLinks(adminNavLinks, input.userPermissions))
  }
  return links
}

function buildAdminNavItems(userPermissions: string[]): NavLink[] {
  const contentChildren = filterAdminLinks(
    [
      { path: '/admin/users', title: '用户管理', icon: User },
      { path: '/admin/roles', title: '角色管理', icon: User },
      { path: '/admin/textures', title: '材质管理', icon: Box },
    ],
    userPermissions,
  )
  const directItems = filterAdminLinks(
    [
      { type: 'item' as const, path: '/admin/invites', title: '邀请码管理', icon: Tools },
      { type: 'item' as const, path: '/admin/settings', title: '站点设置', icon: Setting },
    ],
    userPermissions,
  )
  const configChildren = filterAdminLinks(
    [
      { path: '/admin/email', title: '邮件服务', icon: Message },
      { path: '/admin/notices', title: '通知公告', icon: Bell },
      { path: '/admin/mojang', title: 'Fallback 服务', icon: Link },
      { path: '/admin/homepage-media', title: '首页图片', icon: Picture },
      { path: '/admin/easter-eggs', title: '彩蛋列表', icon: MagicStick },
    ],
    userPermissions,
  )

  return [
    { type: 'item', path: '/dashboard', title: '返回面板', icon: Back },
    ...(contentChildren.length
      ? [
          {
            type: 'group' as const,
            index: 'admin-content-group',
            title: '用户与内容',
            trigger: 'click' as const,
            children: contentChildren,
          },
        ]
      : []),
    ...directItems,
    ...(configChildren.length
      ? [
          {
            type: 'group' as const,
            index: 'admin-config-group',
            title: '更多设置',
            trigger: 'click' as const,
            children: configChildren,
          },
        ]
      : []),
  ]
}

function canAccessSiteLink(item: NavLink | DrawerLink, userPermissions: string[]) {
  return !!item.path && canAccessSitePath(item.path, userPermissions)
}

function canAccessAdminLink(item: NavLink | DrawerLink, userPermissions: string[]) {
  return (
    item.path === '/dashboard' || (!!item.path && canAccessAdminPath(item.path, userPermissions))
  )
}

function filterAdminLinks<T extends NavLink | DrawerLink>(items: T[], userPermissions: string[]) {
  return items.filter((item) => canAccessAdminLink(item, userPermissions))
}
