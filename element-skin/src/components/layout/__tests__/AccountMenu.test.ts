import { computed, createApp, h, nextTick, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import AccountMenu from '../AccountMenu.vue'
import { buildAccountLinks, buildDrawerLinks, canAccessAdmin } from '../appNavigation'
import { navigationCases } from './fixtures/navigationCases'

function mountMenu(initialPermissions: string[]) {
  const permissions = ref(initialPermissions)
  const navigate = vi.fn()
  const logout = vi.fn()
  const root = document.createElement('div')
  const app = createApp({
    setup() {
      const links = computed(() => buildAccountLinks(permissions.value))
      return () =>
        h(AccountMenu, {
          avatarSrc: '',
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
        h('div', slots.default?.()),
  })
  app.component('el-avatar', {
    setup:
      (_, { slots }) =>
      () =>
        h('div', slots.default?.()),
  })
  app.component('el-button', {
    setup:
      (_, { slots }) =>
      () =>
        h('button', slots.default?.()),
  })
  app.mount(root)
  return { root, permissions, navigate, logout, unmount: () => app.unmount() }
}

describe('account navigation permissions', () => {
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
