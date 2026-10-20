import { createHeroScene } from '../../src/composables/useHeroScene'
import { heroImage, heroNextImage, heroPanorama } from '../../src/composables/__tests__/fixtures/heroMedia'

export interface HeroFrame {
  width: number
  height: number
  pixels: number[][]
}

declare global {
  interface Window {
    heroTest: {
      step(at: number): Promise<HeroFrame>
      reset(): void
    }
  }
}

const params = new URLSearchParams(location.search)
document.documentElement.classList.toggle('dark', params.get('theme') === 'dark')
const canvas = document.querySelector<HTMLCanvasElement>('#hero')!
const sample = document.createElement('canvas')
sample.width = 3
sample.height = 1
const ctx = sample.getContext('2d', { willReadFrequently: true })!
const nativeFrame = window.requestAnimationFrame.bind(window)
const callbacks = new Map<number, FrameRequestCallback>()
let sequence = 0
let now = 0

// Control the scene's clock and RAF queue, but render and read real WebGL pixels
// inside a native browser animation frame. No renderer or shader is mocked.
performance.now = () => now
window.requestAnimationFrame = (callback) => {
  callbacks.set(++sequence, callback)
  return sequence
}
window.cancelAnimationFrame = (id) => callbacks.delete(id)

const kind = params.get('kind')
const items = (kind === 'panorama'
  ? [heroPanorama]
  : kind === 'carousel'
    ? [heroImage, heroNextImage]
    : [heroImage]
).map((item) => ({
  ...item,
  overlay_opacity_light: 0,
  overlay_opacity_dark: 0.5,
  start_yaw: 0,
  start_pitch: 0,
  yaw_speed_dps: 0,
  pitch_speed_dps: 0,
}))
const scene = createHeroScene({ maxFps: 10, transition: 1000 })
scene.setTarget(canvas)
scene.setMedia(items)
scene.start()

window.heroTest = {
  reset() {
    scene.stop()
    now = 0
    scene.setMedia(items)
    scene.start()
  },
  step(at) {
    return new Promise((resolve) => {
      nativeFrame(() => {
        now = at
        const scheduled = [...callbacks.values()]
        callbacks.clear()
        scheduled.forEach((callback) => callback(at))
        ctx.clearRect(0, 0, 3, 1)
        for (const [index, x] of [0.1, 0.5, 0.9].entries()) {
          ctx.drawImage(
            canvas,
            Math.floor(canvas.width * x),
            Math.floor(canvas.height / 2),
            1, 1, index, 0, 1, 1,
          )
        }
        const data = ctx.getImageData(0, 0, 3, 1).data
        resolve({
          width: canvas.width,
          height: canvas.height,
          pixels: [0, 4, 8].map((offset) => Array.from(data.slice(offset, offset + 4))),
        })
      })
    })
  },
}
window.addEventListener('pagehide', () => scene.destroy(), { once: true })
