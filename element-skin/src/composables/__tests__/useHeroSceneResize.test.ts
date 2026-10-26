import * as THREE from 'three'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createHeroScene, type HeroSceneController } from '../useHeroScene'
import { heroImage, heroNextImage, heroPanorama, heroResizeViewports } from './fixtures/heroMedia'

const mocks = vi.hoisted(() => ({
  renderers: [] as Array<{
    setDrawingBufferSize: ReturnType<typeof vi.fn>
    setPixelRatio: ReturnType<typeof vi.fn>
    setSize: ReturnType<typeof vi.fn>
    render: ReturnType<typeof vi.fn>
    clear: ReturnType<typeof vi.fn>
    dispose: ReturnType<typeof vi.fn>
  }>,
  loads: [] as Array<() => void>,
  operations: [] as string[],
}))

vi.mock('three', async (importOriginal) => {
  const actual = await importOriginal<typeof import('three')>()
  return {
    ...actual,
    WebGLRenderer: class {
      setDrawingBufferSize = vi.fn(() => mocks.operations.push('resize'))
      setPixelRatio = vi.fn(() => mocks.operations.push('pixelRatio'))
      setSize = vi.fn(() => mocks.operations.push('size'))
      render = vi.fn(() => mocks.operations.push('render'))
      setClearColor = vi.fn()
      clear = vi.fn(() => mocks.operations.push('clear'))
      dispose = vi.fn()
      constructor() {
        mocks.renderers.push(this)
      }
    },
    TextureLoader: class {
      load(_url: string, onLoad: () => void) {
        mocks.loads.push(onLoad)
        return new actual.Texture({ width: 1600, height: 900 })
      }
    },
  }
})

let scene: HeroSceneController
let now: number
let rect: { width: number; height: number }
let frames: Map<number, FrameRequestCallback>
let nextFrameId: number
let images: Array<{ onload: () => void }>

beforeEach(() => {
  mocks.renderers.length = 0
  mocks.loads.length = 0
  mocks.operations.length = 0
  now = 0
  rect = { width: 1280, height: 720 }
  frames = new Map()
  nextFrameId = 0
  images = []
  document.documentElement.classList.remove('dark')
  vi.spyOn(performance, 'now').mockImplementation(() => now)
  vi.stubGlobal('devicePixelRatio', 1)
  vi.spyOn(HTMLCanvasElement.prototype, 'getBoundingClientRect').mockImplementation(
    () => rect as DOMRect,
  )
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    fillRect: vi.fn(),
    translate: vi.fn(),
    scale: vi.fn(),
    drawImage: vi.fn(),
  } as unknown as CanvasRenderingContext2D)
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    frames.set(++nextFrameId, callback)
    return nextFrameId
  })
  vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id))
  vi.stubGlobal(
    'Image',
    class {
      naturalWidth = 256
      naturalHeight = 256
      onload = () => {}
      constructor() {
        images.push(this)
      }
    },
  )
  scene = createHeroScene({ maxFps: 10, transition: 1000 })
  scene.setTarget(document.createElement('canvas'))
  scene.setMedia([heroImage])
  mocks.loads.forEach((load) => load())
})

afterEach(() => {
  scene.destroy()
  vi.unstubAllGlobals()
  document.documentElement.classList.remove('dark')
})

function frame(at: number) {
  now = at
  const scheduled = [...frames.values()]
  frames.clear()
  scheduled.forEach((callback) => callback(at))
}

function resize(width: number, height: number) {
  rect = { width, height }
  window.dispatchEvent(new Event('resize'))
}

function uniforms() {
  const renderedScene = mocks.renderers.at(-1)!.render.mock.lastCall![0] as THREE.Scene
  return (renderedScene.children[0] as THREE.Mesh<THREE.PlaneGeometry, THREE.ShaderMaterial>)
    .material.uniforms
}

describe('hero scene resize scheduling', () => {
  it('preserves the warm draw and renders loaded media before the frame interval elapses', () => {
    expect(mocks.operations).toEqual(['resize', 'render'])
    scene.start()
    frame(1)
    expect(mocks.operations).toEqual(['resize', 'render', 'render'])
    expect(mocks.renderers[0].setDrawingBufferSize.mock.calls).toEqual([[1280, 720, 1]])
    expect(uniforms().viewportAspect.value).toBe(1280 / 720)
  })

  it('coalesces resize events and draws the latest size in the same throttled frame', () => {
    scene.start()
    frame(100)
    mocks.operations.length = 0

    resize(900, 700)
    resize(390, 844)
    expect(mocks.operations).toEqual([])
    frame(101)

    expect(mocks.operations).toEqual(['resize', 'render'])
    expect(mocks.renderers[0].setDrawingBufferSize.mock.calls).toEqual([
      [1280, 720, 1],
      [390, 844, 1],
    ])
    expect(uniforms().viewportAspect.value).toBe(390 / 844)
    expect(uniforms().currentAspect.value).toBe(1600 / 900)
    expect(uniforms().currentMap.value.image).toEqual({ width: 1600, height: 900 })
  })

  it('skips unchanged dimensions and keeps the normal frame limit', () => {
    scene.start()
    frame(100)
    mocks.operations.length = 0
    resize(1280, 720)
    frame(101)
    frame(199)
    expect(mocks.operations).toEqual([])
    frame(200)
    expect(mocks.operations).toEqual(['render'])
    expect(mocks.renderers[0].setDrawingBufferSize.mock.calls).toEqual([[1280, 720, 1]])
  })

  it.each(['light', 'dark'])(
    'redraws every changed viewport during continuous resize in %s mode',
    (theme) => {
      document.documentElement.classList.toggle('dark', theme === 'dark')
      scene.start()
      frame(100)
      const texture = uniforms().currentMap.value
      mocks.operations.length = 0

      for (const [index, viewport] of heroResizeViewports.entries()) {
        resize(viewport.width, viewport.height)
        expect(mocks.operations).toEqual(
          Array.from({ length: index }, () => ['resize', 'render']).flat(),
        )
        frame(101 + index)
        expect(mocks.operations).toEqual(
          Array.from({ length: index + 1 }, () => ['resize', 'render']).flat(),
        )
        expect(mocks.renderers[0].setDrawingBufferSize.mock.lastCall).toEqual([
          viewport.width,
          viewport.height,
          1,
        ])
        expect(uniforms().currentMap.value).toBe(texture)
        expect(uniforms().viewportAspect.value).toBe(viewport.width / viewport.height)
        expect(uniforms().overlayOpacity.value).toBe(theme === 'dark' ? 0.6 : 0.2)
      }
      expect(mocks.renderers[0].clear).not.toHaveBeenCalled()
    },
  )

  it('handles DPR-only changes and skips changes above the capped DPR', () => {
    scene.start()
    frame(100)
    mocks.operations.length = 0
    vi.stubGlobal('devicePixelRatio', 2)
    frame(101)
    expect(mocks.operations).toEqual(['resize', 'render'])
    expect(mocks.renderers[0].setDrawingBufferSize.mock.calls).toEqual([
      [1280, 720, 1],
      [1280, 720, 1.5],
    ])
    mocks.operations.length = 0
    vi.stubGlobal('devicePixelRatio', 3)
    resize(1280, 720)
    frame(102)
    expect(mocks.operations).toEqual([])
  })

  it('rounds fractional sizes and clamps empty bounds without repeated reallocations', () => {
    scene.start()
    frame(100)
    resize(389.1, 843.1)
    frame(101)
    resize(389.9, 843.9)
    frame(102)
    resize(0, 0)
    frame(103)
    expect(mocks.renderers[0].setDrawingBufferSize.mock.calls).toEqual([
      [1280, 720, 1],
      [390, 844, 1],
      [1, 1, 1],
    ])
    expect(uniforms().viewportAspect.value).toBe(1)
  })

  it('does not clear a stopped scene and applies the pending size on restart', () => {
    scene.start()
    frame(100)
    scene.stop()
    mocks.operations.length = 0
    resize(390, 844)
    frame(101)
    expect(mocks.operations).toEqual([])
    expect(frames.size).toBe(0)
    scene.start()
    frame(102)
    expect(mocks.operations).toEqual(['resize', 'render'])
    expect(uniforms().viewportAspect.value).toBe(390 / 844)
  })

  it('warms a replacement canvas, redraws loaded media and releases resize listeners', () => {
    scene.start()
    frame(100)
    scene.setTarget(null)
    frame(101)
    expect(mocks.renderers[0].dispose).toHaveBeenCalledTimes(1)
    scene.setTarget(document.createElement('canvas'))
    frame(102)
    expect(mocks.renderers[1].setDrawingBufferSize.mock.calls).toEqual([[1280, 720, 1]])
    expect(mocks.renderers[1].render).toHaveBeenCalledTimes(2)
    const removeListener = vi.spyOn(window, 'removeEventListener')
    scene.destroy()
    mocks.operations.length = 0
    resize(390, 844)
    frame(103)
    expect(mocks.operations).toEqual([])
    expect(frames.size).toBe(0)
    expect(removeListener.mock.calls.map(([event]) => event)).toEqual(['resize'])
  })

  it.each(['light', 'dark'])('preserves carousel progress while resizing in %s mode', (theme) => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
    scene.setMedia([heroImage, heroNextImage])
    mocks.loads.forEach((load) => load())
    scene.start()
    frame(100)
    const firstTexture = uniforms().currentMap.value
    frame(1000)
    frame(1400)
    expect(uniforms().transitionMix.value).toBeCloseTo(0.32)

    resize(390, 844)
    frame(1500)
    expect(uniforms().currentMap.value).toBe(firstTexture)
    expect(uniforms().transitionMix.value).toBe(0.5)
    expect(uniforms().overlayOpacity.value).toBeCloseTo(theme === 'dark' ? 0.7 : 0.3)
    const nextTexture = uniforms().nextMap.value
    frame(2000)
    expect(uniforms().currentMap.value).toBe(nextTexture)
    expect(uniforms().transitionMix.value).toBe(0)
  })

  it('keeps panorama rotation anchored to the original playback start', () => {
    scene.setMedia([heroPanorama])
    images.forEach((image) => image.onload())
    scene.start()
    frame(100)
    resize(390, 844)
    frame(150)
    const expected = new THREE.Matrix3().setFromMatrix4(
      new THREE.Matrix4().makeRotationFromEuler(
        new THREE.Euler(
          THREE.MathUtils.degToRad(5 + 2 * 0.15),
          THREE.MathUtils.degToRad(10 + 6 * 0.15),
          0,
          'YXZ',
        ),
      ),
    )
    expect(uniforms().currentKind.value).toBe(1)
    expect(uniforms().currentRotation.value.elements).toEqual(expected.elements)
    expect(mocks.operations.slice(-2)).toEqual(['resize', 'render'])
  })
})
