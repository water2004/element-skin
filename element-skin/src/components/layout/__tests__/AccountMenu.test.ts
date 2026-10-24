import { computed, createApp, h, nextTick, reactive, ref } from 'vue'
import { ElAvatar } from 'element-plus'
import { describe, expect, it, vi } from 'vitest'
import AccountMenu from '../AccountMenu.vue'
import { buildAccountLinks, buildDrawerLinks, canAccessAdmin } from '../appNavigation'
import { navigationCases } from './fixtures/navigationCases'

function mountMenu(initialPermissions: string[]) {
  const permissions = ref(initialPermissions)
  const navigate = vi.fn()
  const logout = vi.fn()
  const avatar = reactive({ avatarSrc: '', avatarConfigured: false, avatarLoading: false })
  const root = document.createElement('div')
  const app = createApp({
    setup() {
      const links = computed(() => buildAccountLinks(permissions.value))
      return () =>
        h(AccountMenu, {
          ...avatar,
          accountName: 'Permission test',
          roleLabel: '超级管理员',
          canAccessAdmin: canAccessAdmin(permissions.value),
          links: links.value,
          onNavigate: navigate,
          onLogout: logout,
        })
    },
  })
  app.component('el-popover', {
    setup:
      (_, { slots }) =>
      () =>
        h('div', [slots.reference?.(), slots.default?.()]),
  })
  app.component('el-avatar', ElAvatar)
  app.component('el-button', {
    setup:
      (_, { slots }) =>
      () =>
        h('button', slots.default?.()),
  })
  app.mount(root)
  return { root, avatar, permissions, navigate, logout, unmount: () => app.unmount() }
}

describe('account navigation permissions', () => {
  it('reserves square avatar slots until configured images arrive and handles image failure and retry', async () => {
    const mounted = mountMenu([])
    try {
      mounted.avatar.avatarConfigured = true
      mounted.avatar.avatarLoading = true
      await nextTick()
      const avatars = Array.from(mounted.root.querySelectorAll('.account-avatar'))
      expect(avatars).toHaveLength(2)
      for (const avatar of avatars) {
        expect(avatar.classList.contains('el-avatar--square')).toBe(true)
        expect(avatar.classList.contains('has-custom')).toBe(true)
        expect(avatar.textContent?.trim()).toBe('')
        expect(avatar.getAttribute('aria-busy')).toBe('true')
      }
      expect(avatars[0]!.classList.contains('el-avatar--small')).toBe(true)
      expect((avatars[1] as HTMLElement).style.getPropertyValue('--el-avatar-size')).toBe('48px')

      mounted.avatar.avatarSrc = '/head.png'
      mounted.avatar.avatarLoading = false
      await nextTick()
      expect(Array.from(mounted.root.querySelectorAll('.account-avatar'))).toEqual(avatars)
      expect(
        Array.from(mounted.root.querySelectorAll('img'), (image) => image.getAttribute('src')),
      ).toEqual(['/head.png', '/head.png'])
      expect(avatars.map((avatar) => avatar.textContent?.trim())).toEqual(['', ''])

      mounted.avatar.avatarLoading = true
      await nextTick()
      mounted.root.querySelector('img')!.dispatchEvent(new Event('error'))
      await nextTick()
      expect(mounted.root.querySelectorAll('img')).toHaveLength(0)
      expect(avatars.map((avatar) => avatar.textContent?.trim())).toEqual(['', ''])
      mounted.avatar.avatarLoading = false
      await nextTick()
      mounted.root.querySelector('img')!.dispatchEvent(new Event('error'))
      await nextTick()
      expect(mounted.root.querySelectorAll('img')).toHaveLength(0)
      expect(avatars.map((avatar) => avatar.textContent?.trim())).toEqual(['P', 'P'])

      mounted.avatar.avatarSrc = '/retry.png'
      await nextTick()
      expect(
        Array.from(mounted.root.querySelectorAll('img'), (image) => image.getAttribute('src')),
      ).toEqual(['/retry.png', '/retry.png'])
      expect(avatars.map((avatar) => avatar.textContent?.trim())).toEqual(['', ''])
    } finally {
      mounted.unmount()
    }
  })

  it('shows initials only for absent avatars or a completed render failure', async () => {
    const mounted = mountMenu([])
    try {
      const avatars = Array.from(mounted.root.querySelectorAll('.account-avatar'))
      expect(avatars.map((avatar) => avatar.textContent?.trim())).toEqual(['P', 'P'])
      expect(avatars.every((avatar) => avatar.classList.contains('el-avatar--circle'))).toBe(true)
      mounted.avatar.avatarConfigured = true
      mounted.avatar.avatarLoading = true
      await nextTick()
      expect(avatars.map((avatar) => avatar.textContent?.trim())).toEqual(['', ''])
      mounted.avatar.avatarLoading = false
      await nextTick()
      expect(avatars.map((avatar) => avatar.textContent?.trim())).toEqual(['P', 'P'])
      expect(mounted.root.querySelectorAll('img')).toHaveLength(0)
    } finally {
      mounted.unmount()
    }
  })

  it.each(navigationCases)('matches account and drawer entries for $label', (testCase) => {
    const mounted = mountMenu(testCase.permissions)
    try {
      expect(
        Array.from(mounted.root.querySelectorAll('button'), (button) => button.textContent?.trim()),
      ).toEqual([...testCase.titles, ...(testCase.admin ? ['管理面板'] : []), '退出登录'])
      const drawer = buildDrawerLinks({
        isLogged: true,
        enableSkinLibrary: false,
        userPermissions: testCase.permissions,
      })
      const accountGroup = drawer.find((item) => item.index === 'account-apps-group')
      expect(accountGroup?.children?.map((item) => item.title) ?? []).toEqual([
        ...(testCase.notifications ? ['通知中心'] : []),
        ...testCase.titles,
      ])
      expect(mounted.root.textContent?.includes('账户与应用')).toBe(testCase.titles.length > 0)
      expect(mounted.root.textContent?.includes('面板')).toBe(testCase.admin)
    } finally {
      mounted.unmount()
    }
  })

  it('emits the exact navigation path and logout event', () => {
    const mounted = mountMenu(['oauth_app.create.owned'])
    try {
      mounted.root.querySelectorAll('button')[0]!.click()
      expect(mounted.navigate).toHaveBeenCalledExactlyOnceWith('/dashboard/oauth')
      mounted.root.querySelectorAll('button')[1]!.click()
      expect(mounted.logout).toHaveBeenCalledExactlyOnceWith()
    } finally {
      mounted.unmount()
    }
  })

  it('removes revoked entries and their empty group while retaining logout', async () => {
    const mounted = mountMenu(['external_identity.read.owned', 'oauth_grant.read.owned'])
    try {
      expect(buildAccountLinks(mounted.permissions.value).map((item) => item.path)).toEqual([
        '/dashboard/identities',
        '/dashboard/oauth',
      ])
      mounted.permissions.value = []
      await nextTick()
      expect(
        Array.from(mounted.root.querySelectorAll('button'), (button) => button.textContent?.trim()),
      ).toEqual(['退出登录'])
      expect(mounted.root.textContent).not.toContain('账户与应用')
    } finally {
      mounted.unmount()
    }
  })
})
