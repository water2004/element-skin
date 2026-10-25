import { createApp, h, nextTick, type Ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from '@/api/types'

const mocks = vi.hoisted(() => ({
  getMe: vi.fn(),
  getNotices: vi.fn(),
  siteLogout: vi.fn(),
  initializeAvatar: vi.fn(),
}))
vi.mock('@/api/me', () => ({ getMe: mocks.getMe }))
vi.mock('@/api/auth', () => ({ siteLogout: mocks.siteLogout }))
vi.mock('@/api/notices', () => ({ getNotices: mocks.getNotices }))
vi.mock('@/api/public', () => ({ getPublicSettings: vi.fn(async () => ({ data: {} })) }))
vi.mock('@/composables/useAvatar', () => ({
  useAvatar: () => ({ currentAvatarImg: '', initializeAvatar: mocks.initializeAvatar }),
}))
vi.mock('@/composables/useTheme', () => ({
  useTheme: () => ({ isDark: false, initTheme: vi.fn(), toggleTheme: vi.fn() }),
}))
vi.mock('@/storage', () => ({
  appStorage: {
    cleanupUnusedKeys: vi.fn(),
    siteSettings: { getSiteName: () => 'Session Test', getEnableSkinLibrary: () => true },
  },
}))
vi.mock('@/easter-eggs', () => ({
  cleanupEasterEgg: vi.fn(),
  installEasterEggDevTools: vi.fn(),
  refreshEasterEgg: vi.fn(),
  setServerEasterEggConfig: vi.fn(),
}))

const userFixture = {
  id: 'session-user',
  email: 'session@example.com',
  display_name: 'SessionUser',
  permissions: ['account.update.self', 'account.delete.self', 'notice.read.owned'],
} as User

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
  vi.useFakeTimers()
  mocks.getMe.mockResolvedValue({ data: userFixture })
  mocks.getNotices.mockResolvedValue({ data: { items: [], page_size: 0, has_next: false } })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('local session state', () => {
  it('clears user and navigation state and stops polling without logout or navigation requests', async () => {
    const mounted = await mountState()
    try {
      expect(mounted.user.value).toEqual(userFixture)
      mounted.state.drawer.value = true
      mounted.state.clearSessionState()
      await nextTick()
      expect(mounted.user.value).toBeNull()
      expect(mounted.state.isLogged.value).toBe(false)
      expect(mounted.state.authReady.value).toBe(true)
      expect(mounted.state.drawer.value).toBe(false)
      expect(mocks.initializeAvatar).toHaveBeenNthCalledWith(2, null)
      expect(mocks.initializeAvatar).toHaveBeenCalledTimes(2)
      expect(mounted.notifications.hasUnreadNotifications.value).toBe(false)
      expect(mounted.notifications.loadingUnreadNotifications.value).toBe(false)
      expect(mounted.router.currentRoute.value.path).toBe('/dashboard/profile')
      expect(mocks.getNotices).toHaveBeenCalledTimes(1)
      await vi.advanceTimersByTimeAsync(120_000)
      expect(mocks.getNotices).toHaveBeenCalledTimes(1)
      expect(mocks.siteLogout).not.toHaveBeenCalled()
    } finally {
      mounted.unmount()
    }
  })

  it('ignores a user response that arrives after local session cleanup', async () => {
    const pending = Promise.withResolvers<{ data: User }>()
    mocks.getMe.mockReturnValue(pending.promise)
    const mounted = await mountState()
    try {
      mounted.state.clearSessionState()
      pending.resolve({ data: userFixture })
      await nextTick()
      await Promise.resolve()
      expect(mounted.user.value).toBeNull()
      expect(mounted.state.authReady.value).toBe(true)
      expect(mocks.getMe).toHaveBeenCalledTimes(1)
      expect(mocks.getNotices).not.toHaveBeenCalled()
      expect(mocks.siteLogout).not.toHaveBeenCalled()
    } finally {
      mounted.unmount()
    }
  })

  it('preserves a new session when a user request from before cleanup later fails', async () => {
    const pending = Promise.withResolvers<{ data: User }>()
    mocks.getMe.mockReturnValueOnce(pending.promise).mockResolvedValueOnce({ data: userFixture })
    const mounted = await mountState()
    try {
      mounted.state.clearSessionState()
      await mounted.fetchMe()
      pending.reject(new Error('old user request failed'))
      await nextTick()
      await Promise.resolve()
      expect(mounted.user.value).toEqual(userFixture)
      expect(mounted.state.isLogged.value).toBe(true)
      expect(mounted.state.authReady.value).toBe(true)
      expect(mocks.getMe).toHaveBeenCalledTimes(2)
      expect(mocks.siteLogout).not.toHaveBeenCalled()
    } finally {
      mounted.unmount()
    }
  })
})

async function mountState() {
  const { useAppLayoutState } = await import('../useAppLayoutState')
  const { useNotificationIndicator } = await import('@/composables/useNotificationIndicator')
  let state!: ReturnType<typeof useAppLayoutState>
  const router = createRouter({
    history: createMemoryHistory(),
    routes: ['/dashboard/profile', '/'].map((path) => ({
      path,
      component: { render: () => h('div') },
    })),
  })
  await router.push('/dashboard/profile')
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp({
    setup() {
      state = useAppLayoutState()
      return () => h('div')
    },
  })
  app.use(router)
  app.mount(host)
  await nextTick()
  await Promise.resolve()
  await nextTick()
  const provided = app._instance!.provides
  return {
    state,
    router,
    user: provided.user as Ref<User | null>,
    fetchMe: provided.fetchMe as () => Promise<void>,
    notifications: useNotificationIndicator(),
    unmount() {
      app.unmount()
      host.remove()
    },
  }
}
