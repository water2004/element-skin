import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from '@/api/types'
import NotificationsView from '../NotificationsView.vue'
import { firstDetail, firstNotice, noticesPage, secondDetail } from './fixtures/notifications'

const api = vi.hoisted(() => ({
  getNotice: vi.fn(),
  getNotices: vi.fn(),
  dismissNotice: vi.fn(),
  refreshUnreadNotifications: vi.fn(),
}))
vi.mock('@/api/notices', () => api)
vi.mock('@/composables/useNotificationIndicator', () => ({
  useNotificationIndicator: () => ({ refreshUnreadNotifications: api.refreshUnreadNotifications }),
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((complete, fail) => {
    resolve = complete
    reject = fail
  })
  return { promise, resolve, reject }
}

async function flushUI() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}

async function mountNotices(path = '/notifications', permissions = ['notice.read.owned']) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/notifications/:id?', component: NotificationsView },
      { path: '/', component: { render: () => null } },
    ],
  })
  await router.push(path)
  const user = ref<User>({ id: 'user-1', email: 'test@example.com', permissions })
  const root = document.createElement('div')
  document.body.append(root)
  const app = createApp({ render: () => h(RouterView) })
  app.provide('user', user).use(router).use(ElementPlus)
  app.mount(root)
  await flushUI()
  return {
    root,
    router,
    user,
    unmount: () => {
      app.unmount()
      root.remove()
    },
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      observe() {}
      disconnect() {}
    },
  )
  api.getNotices.mockResolvedValue({ data: noticesPage })
  api.getNotice.mockResolvedValue({ data: firstDetail })
  api.dismissNotice.mockResolvedValue({ data: undefined })
  vi.spyOn(ElMessage, 'error').mockImplementation(() => ({ close: vi.fn() }))
  vi.spyOn(ElMessage, 'success').mockImplementation(() => ({ close: vi.fn() }))
})
afterEach(() => vi.unstubAllGlobals())

describe('notification route and request boundaries', () => {
  it('loads only the list until a detail is explicitly selected', async () => {
    const mounted = await mountNotices()
    try {
      expect(api.getNotices).toHaveBeenCalledExactlyOnceWith({
        cursor: null,
        limit: 20,
        include_read: true,
      })
      expect(api.getNotice).not.toHaveBeenCalled()
      expect(mounted.root.textContent).toContain('选择一条通知查看详情')
      const select = Array.from(mounted.root.querySelectorAll('button')).find((button) =>
        button.textContent?.includes(firstNotice.title),
      )!
      select.click()
      await flushUI()
      expect(mounted.router.currentRoute.value.path).toBe('/notifications/notice-a')
      expect(api.getNotice).toHaveBeenCalledExactlyOnceWith('notice-a')
      expect(mounted.root.querySelector('article')?.textContent).toContain(
        firstDetail.content_markdown,
      )
      expect(api.refreshUnreadNotifications).toHaveBeenCalledTimes(1)
    } finally {
      mounted.unmount()
    }
  })

  it.each(['success', 'failure'])(
    'ignores a late $s after returning to the list',
    async (outcome) => {
      const request = deferred<{ data: typeof firstDetail }>()
      api.getNotice.mockReturnValue(request.promise)
      const mounted = await mountNotices('/notifications/notice-a')
      try {
        expect(api.getNotice).toHaveBeenCalledExactlyOnceWith('notice-a')
        await mounted.router.push('/notifications')
        if (outcome === 'success') request.resolve({ data: firstDetail })
        else request.reject(new Error('old detail failed'))
        await flushUI()
        expect(mounted.router.currentRoute.value.path).toBe('/notifications')
        expect(mounted.root.querySelector('article')).toBeNull()
        expect(mounted.root.textContent).toContain('选择一条通知查看详情')
        expect(api.refreshUnreadNotifications).not.toHaveBeenCalled()
        expect(ElMessage.error).not.toHaveBeenCalled()
      } finally {
        mounted.unmount()
      }
    },
  )

  it.each(['success', 'failure'])(
    'preserves the newer detail when an older request finishes with $s',
    async (outcome) => {
      const old = deferred<{ data: typeof firstDetail }>()
      api.getNotice.mockImplementation((id) =>
        id === 'notice-a' ? old.promise : Promise.resolve({ data: secondDetail }),
      )
      const mounted = await mountNotices('/notifications/notice-a')
      try {
        await mounted.router.push('/notifications/notice-b')
        await flushUI()
        expect(mounted.root.querySelector('article')?.textContent).toContain(
          secondDetail.content_markdown,
        )
        if (outcome === 'success') old.resolve({ data: firstDetail })
        else old.reject(new Error('old detail failed'))
        await flushUI()
        expect(api.getNotice.mock.calls).toEqual([['notice-a'], ['notice-b']])
        expect(mounted.root.querySelector('article')?.textContent).toContain(
          secondDetail.content_markdown,
        )
        expect(mounted.root.querySelector('article')?.textContent).not.toContain(
          firstDetail.content_markdown,
        )
        expect(api.refreshUnreadNotifications).toHaveBeenCalledTimes(1)
        expect(ElMessage.error).not.toHaveBeenCalled()
      } finally {
        mounted.unmount()
      }
    },
  )

  it('keeps the current loading overlay until its request completes', async () => {
    const old = deferred<{ data: typeof firstDetail }>()
    const current = deferred<{ data: typeof secondDetail }>()
    api.getNotice.mockImplementation((id) => (id === 'notice-a' ? old.promise : current.promise))
    const mounted = await mountNotices('/notifications/notice-a')
    try {
      await mounted.router.push('/notifications/notice-b')
      await flushUI()
      old.resolve({ data: firstDetail })
      await flushUI()
      const overlay = mounted.root
        .querySelectorAll('.ui-card')[1]!
        .querySelector('.el-loading-mask') as HTMLElement
      expect(overlay.style.display).not.toBe('none')
      expect(overlay.classList.contains('el-loading-fade-leave-active')).toBe(false)
      expect(mounted.root.querySelector('article')).toBeNull()
      current.resolve({ data: secondDetail })
      await flushUI()
      await vi.waitFor(() => expect(overlay.isConnected).toBe(false))
      expect(mounted.root.querySelector('article')?.textContent).toContain(
        secondDetail.content_markdown,
      )
    } finally {
      mounted.unmount()
    }
  })

  it('does not start another detail request when a slow list arrives', async () => {
    const list = deferred<{ data: typeof noticesPage }>()
    api.getNotices.mockReturnValue(list.promise)
    const mounted = await mountNotices('/notifications/notice-a')
    try {
      list.resolve({ data: noticesPage })
      await flushUI()
      expect(api.getNotice).toHaveBeenCalledExactlyOnceWith('notice-a')
      expect(mounted.root.querySelector('article')?.textContent).toContain(
        firstDetail.content_markdown,
      )
      expect(mounted.root.querySelector('button .bg-\\[var\\(--color-border\\)\\]')).not.toBeNull()
    } finally {
      mounted.unmount()
    }
  })

  it.each(['success', 'failure'])(
    'ignores an older $s when refreshing the same detail',
    async (outcome) => {
      const old = deferred<{ data: typeof firstDetail }>()
      api.getNotice.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ data: firstDetail })
      const mounted = await mountNotices('/notifications/notice-a')
      try {
        Array.from(mounted.root.querySelectorAll('button'))
          .find((button) => button.textContent?.trim() === '刷新')!
          .click()
        await flushUI()
        if (outcome === 'success')
          old.resolve({ data: { ...firstDetail, content_markdown: 'Outdated detail body' } })
        else old.reject(new Error('outdated refresh failed'))
        await flushUI()
        expect(api.getNotice.mock.calls).toEqual([['notice-a'], ['notice-a']])
        expect(mounted.root.querySelector('article')?.textContent).toContain(
          firstDetail.content_markdown,
        )
        expect(mounted.root.querySelector('article')?.textContent).not.toContain(
          'Outdated detail body',
        )
        expect(api.refreshUnreadNotifications).toHaveBeenCalledTimes(1)
        expect(ElMessage.error).not.toHaveBeenCalled()
      } finally {
        mounted.unmount()
      }
    },
  )

  it('loads the next detail after dismissal and ignores the dismissed pending response', async () => {
    const old = deferred<{ data: typeof firstDetail }>()
    api.getNotice.mockImplementation((id) =>
      id === 'notice-a' ? old.promise : Promise.resolve({ data: secondDetail }),
    )
    const mounted = await mountNotices('/notifications/notice-a', [
      'notice.read.owned',
      'notice.dismiss.owned',
    ])
    try {
      Array.from(mounted.root.querySelectorAll('button'))
        .find((button) => button.textContent?.trim() === '忽略')!
        .click()
      await flushUI()
      old.resolve({ data: firstDetail })
      await flushUI()
      expect(api.dismissNotice).toHaveBeenCalledExactlyOnceWith('notice-a')
      expect(mounted.router.currentRoute.value.path).toBe('/notifications/notice-b')
      expect(api.getNotice.mock.calls).toEqual([['notice-a'], ['notice-b']])
      expect(mounted.root.querySelector('article')?.textContent).toContain(
        secondDetail.content_markdown,
      )
      expect(mounted.root.textContent).not.toContain(firstNotice.title)
      expect(api.refreshUnreadNotifications).toHaveBeenCalledTimes(2)
    } finally {
      mounted.unmount()
    }
  })

  it.each(['success', 'failure'])(
    'ignores late detail $s after leaving the page',
    async (outcome) => {
      const request = deferred<{ data: typeof firstDetail }>()
      api.getNotice.mockReturnValue(request.promise)
      const mounted = await mountNotices('/notifications/notice-a')
      try {
        await mounted.router.push('/')
        if (outcome === 'success') request.resolve({ data: firstDetail })
        else request.reject(new Error('unmounted detail failed'))
        await flushUI()
        expect(mounted.root.textContent).toBe('')
        expect(api.refreshUnreadNotifications).not.toHaveBeenCalled()
        expect(ElMessage.error).not.toHaveBeenCalled()
      } finally {
        mounted.unmount()
      }
    },
  )

  it('reports a current detail failure exactly without refreshing unread state', async () => {
    api.getNotice.mockRejectedValue(new Error('current detail failed'))
    const mounted = await mountNotices('/notifications/notice-a')
    try {
      expect(mounted.root.querySelector('article')).toBeNull()
      expect(ElMessage.error).toHaveBeenCalledExactlyOnceWith('加载通知详情失败')
      expect(api.refreshUnreadNotifications).not.toHaveBeenCalled()
    } finally {
      mounted.unmount()
    }
  })

  it('shows dismissal only while the matching permission is present', async () => {
    const mounted = await mountNotices()
    try {
      const dismissButtons = () =>
        Array.from(mounted.root.querySelectorAll('button')).filter(
          (button) => button.textContent?.trim() === '忽略',
        )
      expect(dismissButtons()).toHaveLength(0)
      mounted.user.value.permissions = ['notice.read.owned', 'notice.dismiss.owned']
      await nextTick()
      expect(dismissButtons()).toHaveLength(2)
      dismissButtons()[0]!.click()
      await flushUI()
      expect(api.dismissNotice).toHaveBeenCalledExactlyOnceWith('notice-a')
      mounted.user.value.permissions = ['notice.read.owned']
      await nextTick()
      expect(dismissButtons()).toHaveLength(0)
    } finally {
      mounted.unmount()
    }
  })
})
