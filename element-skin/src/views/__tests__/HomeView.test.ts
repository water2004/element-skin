import { createApp, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import HomeView from '../HomeView.vue'
import { useSessionStore } from '@/stores/session'
import { homeActionCases, homeSettings, homeUser } from './fixtures/home'

const scene = vi.hoisted(() => ({
  setTarget: vi.fn(),
  start: vi.fn(),
  setMedia: vi.fn(),
  destroy: vi.fn(),
}))

vi.mock('@/composables/useHeroScene', () => ({ createHeroScene: () => scene }))
vi.mock('@/api/public', () => ({
  getPublicSettings: vi.fn(async () => ({ data: homeSettings })),
  getPublicHomepageMedia: vi.fn(async () => ({ data: { items: [] } })),
}))

const frames = new Map<number, FrameRequestCallback>()
let frameId = 0
let cleanup: (() => void) | undefined

beforeEach(() => {
  vi.clearAllMocks()
  frames.clear()
  frameId = 0
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    frames.set(++frameId, callback)
    return frameId
  })
  vi.stubGlobal(
    'cancelAnimationFrame',
    vi.fn((id: number) => frames.delete(id)),
  )
})

afterEach(() => {
  cleanup?.()
  cleanup = undefined
  vi.unstubAllGlobals()
})

async function paintFrame() {
  const callbacks = Array.from(frames.values())
  frames.clear()
  for (const callback of callbacks) callback(0)
  await nextTick()
}

async function mountHome(authReady = false, loggedIn = false) {
  const pinia = createPinia()
  const session = useSessionStore(pinia)
  session.authReady = authReady
  session.user = loggedIn ? { ...homeUser } : null
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: HomeView }],
  })
  await router.push('/')

  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(HomeView)
  app.use(pinia)
  app.use(router)
  app.component('ElIcon', { template: '<span><slot /></span>' })
  app.mount(host)
  await nextTick()

  cleanup = () => {
    app.unmount()
    host.remove()
  }

  return { host, session, router }
}

describe('HomeView entrance lifecycle', () => {
  it('starts text entrance only after the initial state has a paint opportunity', async () => {
    const { host } = await mountHome()
    const content = host.querySelector('.hero-content')!

    expect(host.querySelector('.hero-title')?.textContent).toBe(homeSettings.site_name)
    expect(content.classList.contains('is-entered')).toBe(false)
    expect(scene.setTarget).toHaveBeenCalledExactlyOnceWith(host.querySelector('canvas'))
    expect(scene.start).toHaveBeenCalledExactlyOnceWith()

    await paintFrame()
    expect(content.classList.contains('is-entered')).toBe(false)

    await paintFrame()
    expect(content.classList.contains('is-entered')).toBe(true)
    expect(frames.size).toBe(0)
  })

  it('reserves the action row without guest buttons until the session is ready', async () => {
    const { host, session } = await mountHome()
    await paintFrame()
    await paintFrame()

    expect(host.querySelector('.hero-actions')?.getAttribute('aria-busy')).toBe('true')
    expect(host.querySelectorAll('.home-action').length).toBe(0)

    session.user = { ...homeUser }
    session.authReady = true
    await nextTick()

    expect(host.querySelector('.hero-actions')?.getAttribute('aria-busy')).toBe('false')
    expect(
      Array.from(host.querySelectorAll('.home-action'), (button) => button.textContent?.trim()),
    ).toEqual(['进入个人面板'])
    expect(host.querySelector('.home-action-content')?.textContent?.trim()).toBe('进入个人面板')
    expect(host.querySelector('.hero-content')?.classList.contains('is-entered')).toBe(true)
  })

  it('renders the exact guest actions after an anonymous session resolves', async () => {
    const { host, session } = await mountHome()
    session.authReady = true
    await nextTick()

    expect(
      Array.from(host.querySelectorAll('.home-action-content'), (label) =>
        label.textContent?.trim(),
      ),
    ).toEqual(['登录账号', '即刻注册'])
  })

  it.each([false, true])(
    'starts synchronized button entrance after its own initial paint when session ready is %s',
    async (initiallyReady) => {
      const { host, session } = await mountHome(initiallyReady, initiallyReady)
      if (!initiallyReady) {
        await paintFrame()
        await paintFrame()
        session.user = { ...homeUser }
        session.authReady = true
        await nextTick()
      }

      const button = host.querySelector('.home-action')!
      const label = button.querySelector('.home-action-content')!
      expect(label.textContent?.trim()).toBe('进入个人面板')
      expect(button.classList.contains('home-action-enter-from')).toBe(true)
      expect(label.classList.contains('home-action-enter-from')).toBe(false)

      await paintFrame()
      expect(button.classList.contains('home-action-enter-from')).toBe(true)

      await paintFrame()
      expect(button.classList.contains('home-action-enter-from')).toBe(false)
    },
  )

  it('replaces guest actions immediately when the session becomes authenticated', async () => {
    const { host, session } = await mountHome(true)
    expect(
      Array.from(host.querySelectorAll('button'), (button) => button.textContent?.trim()),
    ).toEqual(['登录账号', '即刻注册'])

    session.user = { ...homeUser }
    await nextTick()

    expect(
      Array.from(host.querySelectorAll('button'), (button) => button.textContent?.trim()),
    ).toEqual(['进入个人面板'])
  })

  it.each(homeActionCases)('routes $label to $path', async ({ loggedIn, label, path }) => {
    const { host, router } = await mountHome(true, loggedIn)
    const navigate = vi.spyOn(router, 'push').mockResolvedValue(undefined)
    const button = Array.from(host.querySelectorAll('button')).find(
      (item) => item.textContent?.trim() === label,
    )!
    button.click()

    expect(navigate).toHaveBeenCalledExactlyOnceWith(path)
  })

  it.each([0, 1])('cancels pending entrance after unmount at frame %i', async (paintedFrames) => {
    await mountHome()
    for (let index = 0; index < paintedFrames; index++) await paintFrame()
    expect(frames.size).toBe(1)

    cleanup?.()
    cleanup = undefined

    expect(frames.size).toBe(0)
    expect(cancelAnimationFrame).toHaveBeenCalledExactlyOnceWith(paintedFrames + 1)
    expect(scene.destroy).toHaveBeenCalledExactlyOnceWith()
  })

  it('restarts entrance when returning home with an established session', async () => {
    await mountHome(true, true)
    await paintFrame()
    await paintFrame()
    cleanup?.()
    cleanup = undefined

    const { host } = await mountHome(true, true)
    expect(host.querySelector('.hero-content')?.classList.contains('is-entered')).toBe(false)
    expect(host.querySelectorAll('.home-action').length).toBe(1)
    await paintFrame()
    await paintFrame()
    expect(host.querySelector('.hero-content')?.classList.contains('is-entered')).toBe(true)
  })
})
