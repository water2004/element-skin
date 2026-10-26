import { afterEach, expect, it, vi } from 'vitest'
import { createHeroScene } from '../useHeroScene'

const renderer = vi.hoisted(() => ({
  setDrawingBufferSize: vi.fn(),
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

it.each(['#ffffff', '#1a1a1a'])(
  'uses home background %s for both the warm draw and subsequent empty frames',
  (background) => {
    vi.clearAllMocks()
    document.documentElement.style.setProperty('--color-home-background', background)
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
      expect(colors).toEqual(Array(7).fill(background))
      expect(renderer.render).toHaveBeenCalledTimes(1)
      scene.start()
      frame(100)
      expect(renderer.setClearColor).toHaveBeenCalledExactlyOnceWith(background, 1)
      expect(renderer.clear).toHaveBeenCalledTimes(1)
    } finally {
      scene.destroy()
    }
    expect(renderer.dispose).toHaveBeenCalledTimes(1)
    expect(cancelAnimationFrame).toHaveBeenCalledExactlyOnceWith(1)
  },
)
