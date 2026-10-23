import { createApp, computed, nextTick, ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AppLayout from '@/components/AppLayout.vue'
import {
  headerMeasurements,
  viewportCases,
  type HeaderMeasurements,
} from './fixtures/headerMeasurements'

const mocks = vi.hoisted(() => ({ useAppLayoutState: vi.fn() }))
vi.mock('@/components/layout/useAppLayoutState', () => mocks)

class TestResizeObserver {
  static instances: TestResizeObserver[] = []
  readonly elements = new Set<Element>()
  readonly observe = vi.fn((element: Element) => this.elements.add(element))
  readonly unobserve = vi.fn((element: Element) => this.elements.delete(element))
  readonly disconnect = vi.fn(() => this.elements.clear())

  constructor(readonly callback: ResizeObserverCallback) {
    TestResizeObserver.instances.push(this)
  }

  resize(element: Element) {
    expect(this.elements.has(element)).toBe(true)
    this.callback([], this as unknown as ResizeObserver)
  }
}

let measurements: HeaderMeasurements = headerMeasurements.desktop
const mountedLayouts: Array<() => void> = []

beforeEach(() => {
  vi.clearAllMocks()
  TestResizeObserver.instances = []
  vi.stubGlobal('ResizeObserver', TestResizeObserver)
  vi.stubGlobal('innerWidth', headerMeasurements.desktop.viewport)
  measurements = headerMeasurements.desktop
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
    this: HTMLElement,
  ) {
    if (this.classList.contains('layout-header')) return rectangle(0, measurements.header)
    if (this.classList.contains('header-brand')) return rectangle(0, measurements.brandRight)
    if (this.classList.contains('header-actions'))
      return rectangle(measurements.actionsLeft, measurements.header - measurements.actionsLeft)
    if (this.classList.contains('desktop-nav') || this.classList.contains('el-menu'))
      return rectangle((measurements.header - measurements.nav) / 2, measurements.nav)
    return rectangle(0, 0)
  })
})

afterEach(() => {
  for (const unmount of mountedLayouts.splice(0)) unmount()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('responsive navigation lifecycle', () => {
  it('observes navigation created after entering from a login page without a window resize', async () => {
    const mounted = await mountLayout('/login')
    expect(mounted.host.querySelector('.layout-header')).toBeNull()
    measurements = headerMeasurements.longAccount
    await mounted.router.push('/')
    await flushLayout()
    const { observer, elements } = navigation(mounted.host)
    expect([...observer.elements]).toEqual(elements)
    expect(observer.observe.mock.calls.map(([element]) => element)).toEqual(elements)
    expectNavigation(mounted.host, false)

    measurements = headerMeasurements.desktop
    observer.resize(elements[2]!)
    await flushLayout()
    expectNavigation(mounted.host, true)
  })

  it('observes the initial header and follows account, site-name and navigation width changes', async () => {
    const mounted = await mountLayout('/')
    const { observer, elements } = navigation(mounted.host)
    expect([...observer.elements]).toEqual(elements)
    expectNavigation(mounted.host, true)
    const originalNav = mounted.host.querySelector('.desktop-nav')

    mounted.state.accountName.value = 'A much longer account name'
    measurements = headerMeasurements.longAccount
    await flushLayout()
    expect(mounted.host.querySelector('.account-name')?.textContent).toBe(
      'A much longer account name',
    )
    observer.resize(elements[2]!)
    await flushLayout()
    expectNavigation(mounted.host, false)

    measurements = headerMeasurements.desktop
    observer.resize(elements[2]!)
    await flushLayout()
    expectNavigation(mounted.host, true)
    expect(mounted.host.querySelector('.desktop-nav')).toBe(originalNav)

    mounted.state.siteName.value = 'A much longer site name'
    measurements = headerMeasurements.longSiteName
    await flushLayout()
    expect(mounted.host.querySelector('.logo')?.textContent).toBe('A much longer site name')
    observer.resize(elements[1]!)
    await flushLayout()
    expectNavigation(mounted.host, false)

    measurements = headerMeasurements.desktop
    observer.resize(elements[1]!)
    await flushLayout()
    expectNavigation(mounted.host, true)
    measurements = headerMeasurements.longNavigation
    observer.resize(elements[3]!)
    await flushLayout()
    expectNavigation(mounted.host, false)
    measurements = headerMeasurements.desktop
    observer.resize(elements[3]!)
    await flushLayout()
    expectNavigation(mounted.host, true)
  })

  it('disconnects removed headers and observes replacement nodes across repeated route changes', async () => {
    const mounted = await mountLayout('/')
    const initial = navigation(mounted.host)
    for (const route of ['/login', '/register', '/reset-password']) {
      const oldElements = [...initial.observer.elements]
      await mounted.router.push(route)
      await flushLayout()
      expect(mounted.host.querySelector('.layout-header')).toBeNull()
      expect([...initial.observer.elements]).toEqual([])
      measurements = headerMeasurements.longSiteName
      await mounted.router.push('/')
      await flushLayout()
      const current = navigation(mounted.host)
      expect(current.observer).toBe(initial.observer)
      expect([...current.observer.elements]).toEqual(current.elements)
      for (const oldElement of oldElements) expect(current.elements).not.toContain(oldElement)
      expectNavigation(mounted.host, false)
    }
    expect(initial.observer.observe).toHaveBeenCalledTimes(16)
    const resizeCalls = initial.observer.observe.mock.calls.length
    mounted.unmount()
    expect([...initial.observer.elements]).toEqual([])
    window.dispatchEvent(new Event('resize'))
    await mounted.router.push('/login')
    await flushLayout()
    expect(initial.observer.observe).toHaveBeenCalledTimes(resizeCalls)
  })

  it.each(viewportCases)(
    'measures $label immediately after the header appears',
    async (testCase) => {
      const mounted = await mountLayout('/login')
      measurements = testCase.measurements
      vi.stubGlobal('innerWidth', measurements.viewport)
      await mounted.router.push('/')
      await flushLayout()
      expectNavigation(mounted.host, testCase.visible)
      const { observer, elements } = navigation(mounted.host)
      measurements = headerMeasurements.desktop
      vi.stubGlobal('innerWidth', measurements.viewport)
      window.dispatchEvent(new Event('resize'))
      await flushLayout()
      expectNavigation(mounted.host, true)
      expect([...observer.elements]).toEqual(elements)
    },
  )

  it('removes the exact window listener and disconnects the observer on unmount', async () => {
    const addListener = vi.spyOn(window, 'addEventListener')
    const removeListener = vi.spyOn(window, 'removeEventListener')
    const mounted = await mountLayout('/')
    const { observer } = navigation(mounted.host)
    const disconnectCalls = observer.disconnect.mock.calls.length
    mounted.unmount()
    expect(observer.disconnect).toHaveBeenCalledTimes(disconnectCalls + 1)
    expect([...observer.elements]).toEqual([])
    expect(addListener).toHaveBeenCalledWith('resize', observer.callback)
    expect(removeListener).toHaveBeenCalledWith('resize', observer.callback)
  })
})

function rectangle(left: number, width: number): DOMRect {
  return new DOMRect(left, 0, width, 64)
}

function navigation(host: HTMLElement) {
  const elements = ['.layout-header', '.header-brand', '.header-actions', '.desktop-nav'].map(
    (selector) => host.querySelector(selector)!,
  )
  const observer = TestResizeObserver.instances.find((instance) =>
    instance.elements.has(elements[0]!),
  )
  expect(observer, 'the current header must be observed').toBeDefined()
  return { observer: observer!, elements }
}

function expectNavigation(host: HTMLElement, visible: boolean) {
  const nav = host.querySelector('.desktop-nav')!
  expect(nav.getAttribute('aria-hidden')).toBe(String(!visible))
  expect(nav.classList.contains('is-hidden')).toBe(!visible)
  expect(nav.querySelector('.el-menu')).not.toBeNull()
}

async function flushLayout() {
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

async function mountLayout(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: ['/', '/login', '/register', '/reset-password'].map((route) => ({
      path: route,
      component: { render: () => null },
    })),
  })
  await router.push(path)
  const state = {
    customAvatar: ref(''),
    isDark: ref(false),
    toggleTheme: vi.fn(),
    isHome: computed(() => router.currentRoute.value.path === '/'),
    isAuthPage: computed(() => router.currentRoute.value.path !== '/'),
    siteName: ref('Navigation Test'),
    authReady: ref(true),
    drawer: ref(false),
    footerText: ref(''),
    filingIcp: ref(''),
    filingIcpLink: ref(''),
    filingMps: ref(''),
    filingMpsLink: ref(''),
    isLogged: ref(true),
    canAccessAdmin: ref(false),
    canAccessNotifications: ref(false),
    accountLinks: ref([]),
    defaultOpeneds: ref([]),
    navLinks: ref([{ title: 'Home', path: '/' }]),
    drawerLinks: ref([]),
    activeRoute: computed(() => router.currentRoute.value.path),
    showFooter: ref(false),
    repoUrl: '',
    repoLabel: '',
    accountRoleLabel: ref('用户'),
    accountName: ref('Account'),
    go: vi.fn(),
    logout: vi.fn(),
    shouldShowNotificationBadge: () => false,
  }
  mocks.useAppLayoutState.mockReturnValue(state)
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(AppLayout)
  app.use(router)
  app.use(ElementPlus)
  app.mount(host)
  await flushLayout()
  let isMounted = true
  const unmount = () => {
    if (!isMounted) return
    isMounted = false
    app.unmount()
    host.remove()
  }
  mountedLayouts.push(unmount)
  return { router, host, state, unmount }
}
