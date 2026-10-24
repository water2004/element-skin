import { afterEach, beforeEach, expect, it, vi } from 'vitest'

const boot = vi.hoisted(() => ({
  app: { use: vi.fn(), mount: vi.fn() },
  router: { afterEach: vi.fn(), isReady: vi.fn() },
}))
vi.mock('vue', () => ({ createApp: () => boot.app }))
vi.mock('pinia', () => ({ createPinia: () => ({}) }))
vi.mock('element-plus', () => ({ default: {} }))
vi.mock('../App.vue', () => ({ default: {} }))
vi.mock('../router', () => ({ default: boot.router }))

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
  document.documentElement.className = 'light'
})
afterEach(() => {
  document.documentElement.className = ''
})

it('waits for initial navigation before mounting and changes backgrounds only for confirmed routes', async () => {
  let confirm!: () => void
  boot.router.isReady.mockReturnValue(
    new Promise<void>((resolve) => {
      confirm = resolve
    }),
  )
  await import('../main')
  expect(boot.app.mount).toHaveBeenCalledTimes(0)
  expect(boot.router.isReady).toHaveBeenCalledTimes(1)
  const afterNavigation = boot.router.afterEach.mock.calls[0]![0]
  afterNavigation({ path: '/' }, {}, undefined)
  expect(document.documentElement.className).toBe('light is-home-page')
  afterNavigation({ path: '/dashboard/home' }, {}, { type: 4 })
  expect(document.documentElement.className).toBe('light is-home-page')
  afterNavigation({ path: '/dashboard/home' }, {}, undefined)
  expect(document.documentElement.className).toBe('light')
  confirm()
  await Promise.resolve()
  expect(boot.app.mount).toHaveBeenCalledExactlyOnceWith('#app')
})
