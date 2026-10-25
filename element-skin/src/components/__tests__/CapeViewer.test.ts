import { createApp, nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import CapeViewer from '../CapeViewer.vue'

type BackEquipment = 'cape' | 'elytra'

interface MockViewer {
  canvas: HTMLCanvasElement
  loadCape: ReturnType<typeof vi.fn>
  dispose: ReturnType<typeof vi.fn>
  autoRotate: boolean
  autoRotateSpeed: number
  zoom: number
  playerObject: {
    skin: { visible: boolean }
    backEquipment: BackEquipment | null
  }
  playerWrapper: { position: { y: number } }
}

function makeViewer(): MockViewer {
  return {
    canvas: document.createElement('canvas'),
    loadCape: vi.fn().mockResolvedValue(undefined),
    dispose: vi.fn(),
    autoRotate: false,
    autoRotateSpeed: 0,
    zoom: 0,
    playerObject: {
      skin: { visible: true },
      backEquipment: null,
    },
    playerWrapper: { position: { y: 0 } },
  }
}

const mocks = vi.hoisted(() => ({
  SkinViewer: vi.fn(),
}))

vi.mock('skinview3d', () => mocks)
vi.mock('@/storage/renderCache', () => ({
  canvasToBlob: vi.fn(),
  capeSnapshotCacheKey: vi.fn().mockReturnValue(''),
  getCachedImageUrl: vi.fn().mockResolvedValue(null),
  setCachedImageUrl: vi.fn(),
}))

describe('CapeViewer back equipment switching', () => {
  it('applies the latest equipment selected while the cape texture is loading', async () => {
    const capeLoad = pending<void>()
    const instance = makeViewer()
    instance.loadCape.mockReturnValue(capeLoad.promise)
    useViewer(instance)

    const mounted = mountViewer()
    await flushUI()

    expect(instance.loadCape).toHaveBeenCalledTimes(1)
    expect(instance.loadCape).toHaveBeenCalledWith('https://example.com/cape.png', {
      makeVisible: false,
    })

    mounted.setEquipment('elytra')
    await nextTick()
    expect(instance.playerObject.backEquipment).toBeNull()

    capeLoad.resolve()
    await flushUI()

    expect(instance.playerObject.backEquipment).toBe('elytra')
    expect(instance.loadCape).toHaveBeenCalledTimes(1)
    expect(mounted.host.querySelector('.cape-viewer-container')?.children).toHaveLength(1)
    mounted.unmount()
  })

  it('switches the loaded texture synchronously without loading it again', async () => {
    const instance = makeViewer()
    useViewer(instance)

    const mounted = mountViewer()
    await flushUI()

    expect(instance.playerObject.backEquipment).toBe('cape')

    mounted.setEquipment('elytra')
    await nextTick()
    expect(instance.playerObject.backEquipment).toBe('elytra')

    mounted.setEquipment('cape')
    await nextTick()
    expect(instance.playerObject.backEquipment).toBe('cape')
    expect(instance.loadCape).toHaveBeenCalledTimes(1)
    mounted.unmount()
  })
})

function useViewer(instance: MockViewer) {
  mocks.SkinViewer.mockImplementation(function (this: unknown) {
    return instance
  })
}

function mountViewer() {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(CapeViewer, {
    capeUrl: 'https://example.com/cape.png',
    width: 200,
    height: 280,
  })
  app.mount(host)
  const setup = app._instance?.setupState as unknown as {
    backEquipment: BackEquipment
  }

  return {
    host,
    setEquipment(value: BackEquipment) {
      setup.backEquipment = value
    },
    unmount() {
      app.unmount()
      host.remove()
    },
  }
}

function pending<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

async function flushUI() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}
