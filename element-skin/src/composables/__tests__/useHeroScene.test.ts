import { afterEach, expect, it, vi } from 'vitest'
import { createHeroScene } from '../useHeroScene'

const renderer = vi.hoisted(() => ({
  setPixelRatio: vi.fn(),
  setSize: vi.fn(),
  render: vi.fn(),
  setClearColor: vi.fn(),
  clear: vi.fn(),
  dispose: vi.fn(),
}))
vi.mock('three', async (importOriginal) => ({
  ...(await importOriginal<typeof import('three')>()),
  WebGLRenderer: class {
    constructor() {
      return renderer
    }
  },
}))

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  document.documentElement.style.removeProperty('--color-home-background')
})

it('uses the document home background for both the warm draw and subsequent empty frames', () => {
  document.documentElement.style.setProperty('--color-home-background', '#1a1a1a')
  const colors: string[] = []
  const context = {
    fillStyle: '',
    fillRect: () => {
      colors.push(context.fillStyle)
    },
  }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    context as unknown as CanvasRenderingContext2D,
  )
  let frame!: FrameRequestCallback
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn((callback) => {
      frame = callback
      return 1
    }),
  )
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
  vi.spyOn(performance, 'now').mockReturnValue(100)
  renderer.render.mockImplementation((scene) => {
    expect(scene.children[0].material.uniforms.overlayOpacity.value).toBe(0)
  })

  const scene = createHeroScene()
  try {
    scene.setTarget(document.createElement('canvas'))
    expect(colors).toEqual(Array(7).fill('#1a1a1a'))
    expect(renderer.render).toHaveBeenCalledTimes(1)
    scene.start()
    frame(100)
    expect(renderer.setClearColor).toHaveBeenCalledExactlyOnceWith('#1a1a1a', 1)
    expect(renderer.clear).toHaveBeenCalledTimes(1)
  } finally {
    scene.destroy()
  }
  expect(renderer.dispose).toHaveBeenCalledTimes(1)
  expect(cancelAnimationFrame).toHaveBeenCalledExactlyOnceWith(1)
})
