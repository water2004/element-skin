import { createApp, h, inject, nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAppLayoutState } from '../useAppLayoutState'
import { useNotificationIndicator } from '@/composables/useNotificationIndicator'
import { navigationCases } from './fixtures/navigationCases'

const api = vi.hoisted(() => ({
  getMe: vi.fn(),
  getPublicSettings: vi.fn(),
  getNotices: vi.fn(),
  siteLogout: vi.fn(),
}))
vi.mock('@/api/me', () => ({ getMe: api.getMe }))
vi.mock('@/api/public', () => ({ getPublicSettings: api.getPublicSettings }))
vi.mock('@/api/notices', () => ({ getNotices: api.getNotices }))
vi.mock('@/api/auth', () => ({ siteLogout: api.siteLogout }))
vi.mock('@/composables/useAvatar', () => ({
  useAvatar: () => ({ currentAvatarImg: '', initializeAvatar: vi.fn() }),
}))
vi.mock('@/composables/useTheme', () => ({
  useTheme: () => ({ isDark: false, initTheme: vi.fn(), toggleTheme: vi.fn() }),
}))
vi.mock('@/easter-eggs', () => ({
  cleanupEasterEgg: vi.fn(),
  installEasterEggDevTools: vi.fn(),
  refreshEasterEgg: vi.fn(),
  setServerEasterEggConfig: vi.fn(),
}))

beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  window.localStorage.clear()
  useNotificationIndicator().clearUnreadNotifications()
  api.getPublicSettings.mockResolvedValue({ data: {} })
  api.getNotices.mockResolvedValue({
    data: { items: [], page_size: 0, has_next: false, next_cursor: null },
  })
})
afterEach(() => vi.useRealTimers())

async function flushState() {
  for (let index = 0; index < 5; index++) await nextTick()
}

async function mountLayoutState(permissions: string[]) {
  api.getMe.mockResolvedValue({ data: { id: 'user-1', email: 'test@example.com', permissions } })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { render: () => null } }],
  })
  await router.push('/')
  let state!: ReturnType<typeof useAppLayoutState>
  let fetchMe!: () => Promise<void>
  const child = {
    setup() {
      fetchMe = inject<() => Promise<void>>('fetchMe')!
      return () => h('div')
    },
  }
  const app = createApp({
    setup() {
      state = useAppLayoutState()
      return () => h(child)
    },
  })
  app.use(router)
  app.mount(document.createElement('div'))
  await flushState()
  return { state, fetchMe, unmount: () => app.unmount() }
}

describe('layout notification permissions', () => {
  it.each(navigationCases)(
    'exposes only permitted links and polling for $label',
    async (testCase) => {
      const mounted = await mountLayoutState(testCase.permissions)
      try {
        expect(mounted.state.accountLinks.value.map((link) => link.title)).toEqual(testCase.titles)
        expect(mounted.state.canAccessAdmin.value).toBe(testCase.admin)
        expect(mounted.state.canAccessNotifications.value).toBe(testCase.notifications)
        expect(api.getNotices).toHaveBeenCalledTimes(testCase.notifications ? 1 : 0)
        if (testCase.notifications)
          expect(api.getNotices).toHaveBeenCalledWith({ limit: 1, include_read: false })
        await vi.advanceTimersByTimeAsync(60_000)
        expect(api.getNotices).toHaveBeenCalledTimes(testCase.notifications ? 2 : 0)
      } finally {
        mounted.unmount()
      }
      expect(vi.getTimerCount()).toBe(0)
    },
  )

  it('starts polling when granted and stops it when revoked without logging out', async () => {
    const mounted = await mountLayoutState(['oauth_grant.read.owned'])
    try {
      expect(api.getNotices).not.toHaveBeenCalled()
      api.getMe.mockResolvedValue({
        data: { id: 'user-1', email: 'test@example.com', permissions: ['notice.read.owned'] },
      })
      await mounted.fetchMe()
      await flushState()
      expect(mounted.state.canAccessNotifications.value).toBe(true)
      expect(api.getNotices).toHaveBeenCalledExactlyOnceWith({ limit: 1, include_read: false })
      api.getMe.mockResolvedValue({
        data: { id: 'user-1', email: 'test@example.com', permissions: [] },
      })
      await mounted.fetchMe()
      await flushState()
      expect(mounted.state.isLogged.value).toBe(true)
      expect(mounted.state.canAccessNotifications.value).toBe(false)
      expect(mounted.state.accountLinks.value).toEqual([])
      expect(mounted.state.shouldShowNotificationBadge({ path: '/notifications' })).toBe(false)
      await vi.advanceTimersByTimeAsync(60_000)
      expect(api.getNotices).toHaveBeenCalledTimes(1)
      expect(vi.getTimerCount()).toBe(0)
    } finally {
      mounted.unmount()
    }
  })
})
