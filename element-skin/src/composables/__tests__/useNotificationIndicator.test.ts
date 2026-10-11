import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useNotificationIndicator } from '../useNotificationIndicator'

const api = vi.hoisted(() => ({ getNotices: vi.fn() }))
vi.mock('@/api/notices', () => api)

beforeEach(() => {
  vi.clearAllMocks()
  useNotificationIndicator().clearUnreadNotifications()
})

describe('unread notification request ownership', () => {
  it.each(['success', 'failure'])(
    'ignores stale $s after clearing or switching users',
    async (outcome) => {
      let resolveOld!: (value: unknown) => void
      let rejectOld!: (error: Error) => void
      let resolveCurrent!: (value: unknown) => void
      api.getNotices
        .mockImplementationOnce(
          () =>
            new Promise((resolve, reject) => {
              resolveOld = resolve
              rejectOld = reject
            }),
        )
        .mockImplementationOnce(
          () =>
            new Promise((resolve) => {
              resolveCurrent = resolve
            }),
        )
      const indicator = useNotificationIndicator()
      const oldRequest = indicator.refreshUnreadNotifications()
      indicator.clearUnreadNotifications()
      expect(indicator.hasUnreadNotifications.value).toBe(false)
      expect(indicator.loadingUnreadNotifications.value).toBe(false)
      const currentRequest = indicator.refreshUnreadNotifications()
      if (outcome === 'success') resolveOld({ data: { page_size: 1, has_next: true, items: [] } })
      else rejectOld(new Error('old request failed'))
      await oldRequest
      expect(indicator.hasUnreadNotifications.value).toBe(false)
      expect(indicator.loadingUnreadNotifications.value).toBe(true)
      const sharedRequest = indicator.refreshUnreadNotifications()
      expect(api.getNotices).toHaveBeenCalledTimes(2)
      resolveCurrent({ data: { page_size: 0, has_next: false, items: [] } })
      await Promise.all([currentRequest, sharedRequest])
      expect(indicator.hasUnreadNotifications.value).toBe(false)
      expect(indicator.loadingUnreadNotifications.value).toBe(false)
    },
  )

  it('sets the current badge exactly and clears it on a current failure', async () => {
    const indicator = useNotificationIndicator()
    api.getNotices.mockResolvedValueOnce({ data: { page_size: 1, has_next: false, items: [] } })
    await indicator.refreshUnreadNotifications()
    expect(api.getNotices).toHaveBeenCalledExactlyOnceWith({ limit: 1, include_read: false })
    expect(indicator.hasUnreadNotifications.value).toBe(true)
    api.getNotices.mockRejectedValueOnce(new Error('current request failed'))
    await indicator.refreshUnreadNotifications()
    expect(indicator.hasUnreadNotifications.value).toBe(false)
    expect(indicator.loadingUnreadNotifications.value).toBe(false)
  })
})
