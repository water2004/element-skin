import { createApp, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

import HomeView from '../HomeView.vue'
import { useSessionStore } from '@/stores/session'
import { installMotionStyles, motionCss } from '@/__tests__/helpers/motionStyles'
import { homeActionCases, homeSettings, homeUser } from './fixtures/home'

const scene = vi.hoisted(() => ({
  setTarget: vi.fn(),
  start: vi.fn(),
  setMedia: vi.fn(),
  destroy: vi.fn(),
}))

vi.mock('@/composables/useHeroScene', () => ({ createHeroScene: () => scene }))
const api = vi.hoisted(() => ({ settings: vi.fn(), media: vi.fn(), me: vi.fn() }))
vi.mock('@/api/public', () => ({
  getPublicSettings: api.settings,
  getPublicHomepageMedia: api.media,
}))
vi.mock('@/api/me', () => ({ getMe: api.me }))
let styles: ReturnType<typeof installMotionStyles>
let resetReduced: (() => void) | undefined
beforeAll(() => {
  styles = installMotionStyles({
    source: 'src/views/HomeView.vue',
    scopeId: (HomeView as { __scopeId: string }).__scopeId,
  })
})
afterAll(() => styles.remove())

const frames = new Map<number, FrameRequestCallback>()
let frameId = 0
let cleanup: (() => void) | undefined

beforeEach(() => {
  vi.clearAllMocks()
  api.settings.mockResolvedValue({ data: homeSettings })
  api.media.mockResolvedValue({ data: { items: [] } })
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
  resetReduced?.()
  resetReduced = undefined
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
    const { host } = await mountHome(true)
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
    expect(host.querySelector('.hero-content')?.classList.contains('is-entered')).toBe(false)
    await paintFrame()
    await paintFrame()
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
    'starts copy and actions on the same frame when session ready is %s',
    async (initiallyReady) => {
      const { host, session } = await mountHome(initiallyReady, initiallyReady)
      if (!initiallyReady) {
        expect(frames.size).toBe(0)
        await paintFrame()
        await paintFrame()
        expect(host.querySelector('.hero-content')!.classList.contains('is-entered')).toBe(false)
        session.user = { ...homeUser }
        session.authReady = true
        await nextTick()
      }
      const content = host.querySelector('.hero-content')!
      const copy = host.querySelector('.hero-copy')!
      const label = host.querySelector('.home-action-content')!
      expect(content.classList.contains('is-entered')).toBe(false)
      expect(motionCss(content, 'opacity')).toBe('1')
      expect(motionCss(content, 'translate')).toBe('0 8px')
      expect(motionCss(copy, 'opacity')).toBe('0')
      expect(motionCss(label, 'opacity')).toBe('0')
      expect(motionCss(copy, 'transition')).toBe(motionCss(label, 'transition'))
      await paintFrame()
      expect(content.classList.contains('is-entered')).toBe(false)
      await paintFrame()
      expect(content.classList.contains('is-entered')).toBe(true)
      expect(motionCss(content, 'translate')).toBe('0 0')
      expect(motionCss(copy, 'opacity')).toBe('1')
      expect(motionCss(label, 'opacity')).toBe('1')
      expect(frames.size).toBe(0)
    },
  )

  it('enters after a failed session request without waiting for settings or media', async () => {
    api.settings.mockReturnValue(new Promise(() => {}))
    api.me.mockRejectedValue(new Error('session unavailable'))
    const { host, session } = await mountHome()
    await session.fetchMe()
    await nextTick()
    expect(session.authReady).toBe(true)
    await paintFrame()
    await paintFrame()
    expect(host.querySelector('.hero-content')!.classList.contains('is-entered')).toBe(true)
    expect(
      [...host.querySelectorAll('button')].map((button) => button.textContent?.trim()),
    ).toEqual(['登录账号', '即刻注册'])
  })

  it('keeps copy and late actions visible with reduced motion, including preference changes', async () => {
    resetReduced = styles.reduce()
    const { host, session } = await mountHome()
    const content = host.querySelector('.hero-content')!
    expect(motionCss(content, 'translate')).toBe('none')
    expect(motionCss(host.querySelector('.hero-copy')!, 'opacity')).toBe('1')
    session.authReady = true
    await nextTick()
    expect(motionCss(host.querySelector('.home-action-content')!, 'opacity')).toBe('1')
    expect(content.classList.contains('is-entered')).toBe(false)
    await paintFrame()
    await paintFrame()
    resetReduced()
    resetReduced = undefined
    expect(motionCss(host.querySelector('.hero-copy')!, 'opacity')).toBe('1')
    expect(motionCss(host.querySelector('.home-action-content')!, 'opacity')).toBe('1')
    expect(content.classList.contains('is-entered')).toBe(true)
  })

  it('replaces guest actions immediately when the session becomes authenticated', async () => {
    const { host, session } = await mountHome(true)
    await paintFrame()
    await paintFrame()
    expect(
      Array.from(host.querySelectorAll('button'), (button) => button.textContent?.trim()),
    ).toEqual(['登录账号', '即刻注册'])

    session.user = { ...homeUser }
    await nextTick()

    expect(
      Array.from(host.querySelectorAll('button'), (button) => button.textContent?.trim()),
    ).toEqual(['进入个人面板'])
    expect(frames.size).toBe(0)
    expect(motionCss(host.querySelector('.home-action-content')!, 'opacity')).toBe('1')
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
    await mountHome(true)
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
