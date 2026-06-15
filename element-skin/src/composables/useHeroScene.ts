import type { InjectionKey } from 'vue'
import * as THREE from 'three'
import type { HomepageMedia } from '@/api/types'

export interface HeroSceneController {
  setTarget(canvas: HTMLCanvasElement | null): void
  setMedia(items: HomepageMedia[]): void
  subscribe(fn: () => void): () => void
  getCanvas(): HTMLCanvasElement | null
  getDpr(): number
  start(): void
  stop(): void
  destroy(): void
}

export const heroSceneKey: InjectionKey<HeroSceneController> = Symbol('heroScene')

export interface HeroSceneOptions {
  transition?: number
}

type PreparedMedia =
  | {
      item: HomepageMedia
      kind: 'image'
      texture: THREE.Texture
    }
  | {
      item: HomepageMedia
      kind: 'panorama'
      texture: THREE.CubeTexture
    }

export function createHeroScene(options: HeroSceneOptions = {}): HeroSceneController {
  const transition = options.transition ?? 900

  let target: HTMLCanvasElement | null = null
  let renderer: THREE.WebGLRenderer | null = null
  let dpr = Math.max(window.devicePixelRatio || 1, 1)
  let cssW = 1
  let cssH = 1

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(70, 1, 0.1, 10)
  const quadCamera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1)
  const imageScene = new THREE.Scene()
  const imageMesh = new THREE.Mesh(
    new THREE.PlaneGeometry(2, 2),
    new THREE.MeshBasicMaterial({
      color: 0xffffff,
      depthTest: false,
      depthWrite: false,
      transparent: true,
    }),
  )
  imageScene.add(imageMesh)
  const panoramaUniforms = {
    envMap: { value: null as THREE.CubeTexture | null },
    opacity: { value: 1 },
  }
  const panoramaMaterial = new THREE.ShaderMaterial({
    uniforms: panoramaUniforms,
    vertexShader: `
      varying vec3 vWorldDirection;
      void main() {
        vec4 worldPosition = modelMatrix * vec4(position, 1.0);
        vWorldDirection = normalize(worldPosition.xyz - cameraPosition);
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: `
      uniform samplerCube envMap;
      uniform float opacity;
      varying vec3 vWorldDirection;
      void main() {
        gl_FragColor = vec4(textureCube(envMap, vWorldDirection).rgb, opacity);
      }
    `,
    depthTest: false,
    depthWrite: false,
    side: THREE.BackSide,
    transparent: true,
  })
  const panoramaMesh = new THREE.Mesh(new THREE.BoxGeometry(10, 10, 10), panoramaMaterial)
  scene.add(panoramaMesh)
  const overlayScene = new THREE.Scene()
  const overlayMaterial = new THREE.MeshBasicMaterial({
    color: 0x000000,
    transparent: true,
    opacity: 0.45,
    depthTest: false,
    depthWrite: false,
  })
  const overlayMesh = new THREE.Mesh(
    new THREE.PlaneGeometry(2, 2),
    overlayMaterial,
  )
  overlayScene.add(overlayMesh)

  let media: HomepageMedia[] = []
  const prepared = new Map<string, PreparedMedia>()
  const textureLoader = new THREE.TextureLoader()
  const cubeLoader = new THREE.CubeTextureLoader()

  let current = 0
  let next = 0
  let transitioning = false
  let transStart = 0
  let itemStart = performance.now()
  let rafId = 0
  let running = false
  let listenersBound = false
  const consumers = new Set<() => void>()

  function mediaUrl(item: HomepageMedia, face?: string) {
    const base = import.meta.env.BASE_URL
    const suffix = face ? `${item.storage_path}/${face}` : item.storage_path
    return `${base}static/carousel/${suffix}`.replace(/\/+/g, '/')
  }

  function prepare(item: HomepageMedia) {
    if (prepared.has(item.id)) return
    if (item.type === 'panorama') {
      // CubeTextureLoader expects +X, -X, +Y, -Y, +Z, -Z.
      // Uploaded panorama files are front, right, back, left, up, down.
      const urls = [
        mediaUrl(item, 'panorama_3.png'),
        mediaUrl(item, 'panorama_1.png'),
        mediaUrl(item, 'panorama_4.png'),
        mediaUrl(item, 'panorama_5.png'),
        mediaUrl(item, 'panorama_2.png'),
        mediaUrl(item, 'panorama_0.png'),
      ]
      const texture = cubeLoader.load(urls)
      prepared.set(item.id, { item, kind: 'panorama', texture })
      return
    }
    const texture = textureLoader.load(mediaUrl(item))
    texture.colorSpace = THREE.SRGBColorSpace
    prepared.set(item.id, { item, kind: 'image', texture })
  }

  function resize() {
    if (!target || !renderer) return
    const rect = target.getBoundingClientRect()
    cssW = Math.max(Math.ceil(rect.width), 1)
    cssH = Math.max(Math.ceil(rect.height), 1)
    dpr = Math.max(window.devicePixelRatio || 1, 1)
    renderer.setPixelRatio(dpr)
    renderer.setSize(cssW, cssH, false)
    camera.aspect = cssW / cssH
    camera.updateProjectionMatrix()
  }

  function renderPrepared(entry: PreparedMedia, now: number, alpha: number, startedAt: number) {
    if (!renderer) return
    if (entry.kind === 'panorama') {
      const startYaw = numberConfig(entry.item, 'start_yaw', 0)
      const startPitch = numberConfig(entry.item, 'start_pitch', 0)
      const yawSpeed = numberConfig(entry.item, 'yaw_speed_dps', 4)
      const pitchSpeed = numberConfig(entry.item, 'pitch_speed_dps', 0)
      const elapsed = (now - startedAt) / 1000
      const yaw = THREE.MathUtils.degToRad(startYaw + yawSpeed * elapsed)
      const pitch = THREE.MathUtils.degToRad(startPitch + pitchSpeed * elapsed)
      camera.rotation.set(pitch, yaw, 0, 'YXZ')
      panoramaUniforms.envMap.value = entry.texture
      panoramaUniforms.opacity.value = alpha
      renderer.render(scene, camera)
    } else {
      camera.rotation.set(0, 0, 0)
      const material = imageMesh.material as THREE.MeshBasicMaterial
      material.map = entry.texture
      material.opacity = alpha
      fitImageMesh(entry.texture)
      renderer.render(imageScene, quadCamera)
    }
  }

  function fitImageMesh(texture: THREE.Texture) {
    const image = texture.image as HTMLImageElement | undefined
    const iw = image?.naturalWidth || image?.width || 16
    const ih = image?.naturalHeight || image?.height || 9
    const view = cssW / cssH
    const img = iw / ih
    imageMesh.scale.set(img > view ? img / view : 1, img > view ? 1 : view / img, 1)
  }

  function render(now: number) {
    if (!renderer) return
    if (media.length > 0) {
      const active = media[current]
      const duration = Math.max(active?.duration_ms || 6000, 1000)
      if (media.length > 1 && !transitioning && now - itemStart >= duration) {
        next = (current + 1) % media.length
        transitioning = true
        transStart = now
      }
    }

    renderer.autoClear = true
    renderer.clear()
    const currentItem = media[current]
    const currentEntry = currentItem ? prepared.get(currentItem.id) : null
    if (currentEntry) {
      renderPrepared(currentEntry, now, 1, itemStart)
    } else {
      renderFallback()
    }

    if (transitioning) {
      const progress = easeInOut(Math.min((now - transStart) / transition, 1))
      const nextItem = media[next]
      const nextEntry = nextItem ? prepared.get(nextItem.id) : null
      if (nextEntry) {
        renderer.autoClear = false
        renderPrepared(nextEntry, now, progress, transStart)
      }
      if (progress >= 1) {
        current = next
        transitioning = false
        itemStart = transStart
      }
    }

    overlayMaterial.opacity = overlayOpacity(currentItem, transitioning ? media[next] : undefined, transitioning ? now : undefined)
    renderer.autoClear = false
    renderer.render(overlayScene, quadCamera)
    renderer.autoClear = true
    for (const fn of consumers) fn()
  }

  function renderFallback() {
    if (!renderer) return
    renderer.setClearColor(0x1a1a1a, 1)
    renderer.clear()
  }

  function overlayOpacity(currentItem?: HomepageMedia, nextItem?: HomepageMedia, now?: number) {
    const from = currentItem ? numberConfig(currentItem, 'overlay_opacity', 0.45) : 0.45
    if (!transitioning || !nextItem || now === undefined) return from
    const to = numberConfig(nextItem, 'overlay_opacity', 0.45)
    const progress = easeInOut(Math.min((now - transStart) / transition, 1))
    return lerp(from, to, progress)
  }

  function loop() {
    if (!running) return
    render(performance.now())
    rafId = requestAnimationFrame(loop)
  }

  function bindListeners() {
    if (listenersBound) return
    window.addEventListener('resize', resize)
    listenersBound = true
  }

  function unbindListeners() {
    if (!listenersBound) return
    window.removeEventListener('resize', resize)
    listenersBound = false
  }

  return {
    setTarget(canvas) {
      target = canvas
      if (renderer) {
        renderer.dispose()
        renderer = null
      }
      if (canvas) {
        renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: false })
        resize()
      }
    },
    setMedia(items) {
      media = items.filter((item) => item.enabled)
      for (const item of media) prepare(item)
      current = 0
      next = 0
      transitioning = false
      itemStart = performance.now()
    },
    subscribe(fn) {
      consumers.add(fn)
      return () => consumers.delete(fn)
    },
    getCanvas: () => target,
    getDpr: () => dpr,
    start() {
      if (running) return
      running = true
      bindListeners()
      itemStart = performance.now()
      rafId = requestAnimationFrame(loop)
    },
    stop() {
      running = false
      cancelAnimationFrame(rafId)
    },
    destroy() {
      this.stop()
      unbindListeners()
      consumers.clear()
      for (const entry of prepared.values()) entry.texture.dispose()
      prepared.clear()
      renderer?.dispose()
      renderer = null
      target = null
    },
  }
}

function numberConfig(item: HomepageMedia, key: string, fallback: number) {
  const value = item.config?.[key]
  return typeof value === 'number' ? value : fallback
}

function lerp(a: number, b: number, t: number) {
  return a + (b - a) * t
}

function easeInOut(t: number) {
  return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2
}
