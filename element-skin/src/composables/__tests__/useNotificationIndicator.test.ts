import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ getNotices: vi.fn() }))
vi.mock('@/api/notices', () => mocks)

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

const unreadPage = { data: { items: [{ id: 'notice' }], page_size: 1, has_next: false } }

describe('notification session cleanup', () => {
  it('ignores a response that arrives after notification state is cleared', async () => {
    const pending = Promise.withResolvers<typeof unreadPage>()
    mocks.getNotices.mockReturnValue(pending.promise)
    const { useNotificationIndicator } = await import('../useNotificationIndicator')
    const state = useNotificationIndicator()
    const refreshing = state.refreshUnreadNotifications()
    expect(state.loadingUnreadNotifications.value).toBe(true)
    state.clearUnreadNotifications()
    expect(state.hasUnreadNotifications.value).toBe(false)
    expect(state.loadingUnreadNotifications.value).toBe(false)
    pending.resolve(unreadPage)
    await refreshing
    expect(state.hasUnreadNotifications.value).toBe(false)
    expect(state.loadingUnreadNotifications.value).toBe(false)
    expect(mocks.getNotices).toHaveBeenCalledExactlyOnceWith({ limit: 1, include_read: false })
  })

  it('starts a new refresh after cleanup and retains its deduplication when an old response finishes', async () => {
    const oldRequest = Promise.withResolvers<typeof unreadPage>()
    const newRequest = Promise.withResolvers<typeof unreadPage>()
    mocks.getNotices.mockReturnValueOnce(oldRequest.promise).mockReturnValueOnce(newRequest.promise)
    const { useNotificationIndicator } = await import('../useNotificationIndicator')
    const state = useNotificationIndicator()
    const oldRefresh = state.refreshUnreadNotifications()
    state.clearUnreadNotifications()
    const newRefresh = state.refreshUnreadNotifications()
    oldRequest.resolve(unreadPage)
    await oldRefresh
    expect(state.hasUnreadNotifications.value).toBe(false)
    expect(state.loadingUnreadNotifications.value).toBe(true)
    const duplicate = state.refreshUnreadNotifications()
    expect(mocks.getNotices).toHaveBeenCalledTimes(2)
    expect(mocks.getNotices).toHaveBeenNthCalledWith(1, { limit: 1, include_read: false })
    expect(mocks.getNotices).toHaveBeenNthCalledWith(2, { limit: 1, include_read: false })
    newRequest.resolve(unreadPage)
    await Promise.all([newRefresh, duplicate])
    expect(state.hasUnreadNotifications.value).toBe(true)
    expect(state.loadingUnreadNotifications.value).toBe(false)
  })

  it('ignores an old failure after the new session has loaded unread notifications', async () => {
    const oldRequest = Promise.withResolvers<typeof unreadPage>()
    mocks.getNotices.mockReturnValueOnce(oldRequest.promise).mockResolvedValueOnce(unreadPage)
    const { useNotificationIndicator } = await import('../useNotificationIndicator')
    const state = useNotificationIndicator()
    const oldRefresh = state.refreshUnreadNotifications()
    state.clearUnreadNotifications()
    await state.refreshUnreadNotifications()
    oldRequest.reject(new Error('old request failed'))
    await oldRefresh
    expect(state.hasUnreadNotifications.value).toBe(true)
    expect(state.loadingUnreadNotifications.value).toBe(false)
    expect(mocks.getNotices).toHaveBeenCalledTimes(2)
  })
})
