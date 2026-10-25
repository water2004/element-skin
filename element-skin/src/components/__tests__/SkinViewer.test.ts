import { createApp, h, nextTick, reactive } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import SkinViewer from '../SkinViewer.vue'

interface MockViewer {
  canvas: HTMLCanvasElement
  loadSkin: ReturnType<typeof vi.fn>
  loadCape: ReturnType<typeof vi.fn>
  dispose: ReturnType<typeof vi.fn>
  autoRotate: boolean
  autoRotateSpeed: number
  zoom: number
  animation: { speed: number } | null
  playerObject: {
    skin: {
      leftArm: { rotation: { z: number } }
      rightArm: { rotation: { z: number } }
      leftLeg: { rotation: { z: number } }
      rightLeg: { rotation: { z: number } }
    }
  }
  camera: { position: { set: (x: number, y: number, z: number) => void }; lookAt: (x: number, y: number, z: number) => void }
  render: ReturnType<typeof vi.fn>
}

function makeViewer(): MockViewer {
  return {
    canvas: document.createElement('canvas'),
    loadSkin: vi.fn().mockResolvedValue(undefined),
    loadCape: vi.fn().mockResolvedValue(undefined),
    dispose: vi.fn(),
    autoRotate: false,
    autoRotateSpeed: 0,
    zoom: 0,
    animation: null,
    playerObject: {
      skin: {
        leftArm: { rotation: { z: 0 } },
        rightArm: { rotation: { z: 0 } },
        leftLeg: { rotation: { z: 0 } },
        rightLeg: { rotation: { z: 0 } },
      },
    },
    camera: { position: { set: vi.fn() }, lookAt: vi.fn() },
    render: vi.fn(),
  }
}

// The mock implementations MUST be regular functions: the component constructs
// via `new skinview3d.SkinViewer(...)`, which vitest fulfills with
// Reflect.construct — arrow functions are not constructors and would throw.
const mocks = vi.hoisted(() => ({
  SkinViewer: vi.fn().mockImplementation(function (
    this: unknown,
    opts: { canvas?: HTMLCanvasElement },
  ) {
    const instance = makeViewer()
    if (opts?.canvas) instance.canvas = opts.canvas
    return instance
  }),
  WalkingAnimation: vi.fn(),
}))

vi.mock('skinview3d', () => mocks)
vi.mock('@/storage/renderCache', () => ({
  canvasToBlob: vi.fn(),
  getCachedImageUrl: vi.fn().mockResolvedValue(null),
  setCachedImageUrl: vi.fn(),
  skinSnapshotCacheKey: vi.fn().mockReturnValue(''),
}))

async function flushUI() {
  await Promise.resolve()
  await new Promise((r) => setTimeout(r, 0))
  await nextTick()
}

function pending<T>(): { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void } {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  return { promise: new Promise<T>((res, rej) => { resolve = res; reject = rej }), resolve, reject }
}

function overrideViewer(factory: (opts?: { canvas?: HTMLCanvasElement }) => MockViewer) {
  mocks.SkinViewer.mockImplementation(function (this: unknown, opts?: { canvas?: HTMLCanvasElement }) {
    return factory(opts)
  })
}

function mountViewer(overrides: Record<string, unknown> = {}) {
  const state = reactive({
    skinUrl: 'https://example.com/skin.png',
    capeUrl: null as string | null,
    model: 'default',
    isStatic: false,
    ...overrides,
  })
  const errorEvents: unknown[] = []
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp({
    render: () => h(SkinViewer, { ...state, onError: (e: unknown) => errorEvents.push(e) }),
  })
  app.mount(host)
  return { host, state, errorEvents, unmount: () => { app.unmount(); host.remove() } }
}

describe('SkinViewer async lifecycle', () => {
  it('mounts the viewer canvas after loadSkin resolves', async () => {
    mocks.SkinViewer.mockClear()
    const mounted = mountViewer()
    await flushUI()

    const results = mocks.SkinViewer.mock.results
    expect(results).toHaveLength(1)
    expect(results[0].type).toBe('return')
    const instance = results[0].value as MockViewer
    expect(instance.loadSkin).toHaveBeenCalledWith('https://example.com/skin.png', { model: 'default' })
    expect(instance.dispose).not.toHaveBeenCalled()
    expect(mounted.host.querySelector('.skin-viewer-container')?.children.length).toBe(1)
    mounted.unmount()
  })

  it('disposes the stale viewer and skips mounting when unmounted during loadSkin', async () => {
    mocks.SkinViewer.mockClear()
    const skinLoad = pending<void>()
    let created: MockViewer | null = null
    overrideViewer(() => {
      const v = makeViewer()
      v.loadSkin.mockReturnValue(skinLoad.promise)
      created = v
      return v
    })

    const mounted = mountViewer()
    await flushUI()

    const results = mocks.SkinViewer.mock.results
    expect(results).toHaveLength(1)
    expect(results[0].type).toBe('return')
    expect(mounted.host.querySelector('.skin-viewer-container')?.children.length).toBe(0)

    mounted.unmount()
    skinLoad.resolve()
    await flushUI()

    expect(created?.dispose).toHaveBeenCalled()
  })

  it('only mounts the latest viewer when props change during loadSkin', async () => {
    mocks.SkinViewer.mockClear()
    const firstLoad = pending<void>()
    const secondLoad = pending<void>()
    let callCount = 0
    const created: MockViewer[] = []
    overrideViewer(() => {
      const v = makeViewer()
      callCount++
      v.loadSkin.mockReturnValue(callCount === 1 ? firstLoad.promise : secondLoad.promise)
      created.push(v)
      return v
    })

    const mounted = mountViewer()
    await flushUI()
    expect(mocks.SkinViewer.mock.results).toHaveLength(1)

    mounted.state.skinUrl = 'https://example.com/skin-b.png'
    await flushUI()
    expect(mocks.SkinViewer.mock.results).toHaveLength(2)
    // The second initViewer cannot dispose the first instance yet: the module
    // viewer is still null until the first load finishes, so the first one is
    // disposed by its own stale check once its load resolves.
    expect(created[0].dispose).not.toHaveBeenCalled()

    firstLoad.resolve()
    await flushUI()
    expect(created[0].dispose).toHaveBeenCalled()
    expect(mounted.host.querySelector('.skin-viewer-container')?.children.length).toBe(0)

    secondLoad.resolve()
    await flushUI()
    expect(mounted.host.querySelector('.skin-viewer-container')?.children.length).toBe(1)
    expect(created[1].dispose).not.toHaveBeenCalled()
    mounted.unmount()
  })

  it('emits error and disposes the viewer when loadSkin rejects', async () => {
    mocks.SkinViewer.mockClear()
    let created: MockViewer | null = null
    overrideViewer(() => {
      const v = makeViewer()
      v.loadSkin.mockRejectedValueOnce(new Error('corrupt texture'))
      created = v
      return v
    })

    const mounted = mountViewer({ skinUrl: 'https://example.com/bad.png' })
    await flushUI()
    await flushUI()

    expect(mounted.errorEvents).toHaveLength(1)
    expect(String(mounted.errorEvents[0])).toContain('corrupt texture')
    expect(created?.dispose).toHaveBeenCalled()
    expect(mounted.host.querySelector('.skin-viewer-container')?.children.length).toBe(0)
    mounted.unmount()
  })

  it('does not emit error from a stale task after a newer load has started', async () => {
    mocks.SkinViewer.mockClear()
    const firstLoad = pending<void>()
    let secondResolved = false
    let callCount = 0
    overrideViewer(() => {
      const v = makeViewer()
      callCount++
      if (callCount === 1) {
        v.loadSkin.mockReturnValue(firstLoad.promise)
      } else {
        v.loadSkin.mockImplementation(async () => { secondResolved = true })
      }
      return v
    })

    const mounted = mountViewer({ skinUrl: 'https://example.com/a.png' })
    await flushUI()

    mounted.state.skinUrl = 'https://example.com/b.png'
    await flushUI()

    firstLoad.reject(new Error('stale failure'))
    await flushUI()
    await flushUI()

    expect(mounted.errorEvents).toHaveLength(0)
    expect(secondResolved).toBe(true)
    mounted.unmount()
  })
})