import * as THREE from 'three'
import type { HomepageMedia } from '@/api/types'

export interface HeroSceneController {
  setTarget(canvas: HTMLCanvasElement | null): void
  setMedia(items: HomepageMedia[]): void
  start(): void
  stop(): void
  destroy(): void
}

export interface HeroSceneOptions {
  transition?: number
  maxFps?: number
}

type PreparedMedia =
  | {
      item: HomepageMedia
      kind: 'image'
      texture: THREE.Texture
      ready: boolean
    }
  | {
      item: HomepageMedia
      kind: 'panorama'
      texture: THREE.CubeTexture
      ready: boolean
    }

export function createHeroScene(options: HeroSceneOptions = {}): HeroSceneController {
  const transition = options.transition ?? 900
  const minFrameInterval = 1000 / (options.maxFps ?? 60)

  let target: HTMLCanvasElement | null = null
  let renderer: THREE.WebGLRenderer | null = null
  let dpr = sceneDpr()
  let cssW = 1
  let cssH = 1

  const quadCamera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1)
  const compositeScene = new THREE.Scene()
  const fallbackTexture = createFallbackTexture()
  const fallbackCubeTexture = createFallbackCubeTexture()
  const compositeUniforms: {
    currentMap: { value: THREE.Texture }
    nextMap: { value: THREE.Texture }
    currentCube: { value: THREE.CubeTexture }
    nextCube: { value: THREE.CubeTexture }
    currentKind: { value: number }
    nextKind: { value: number }
    currentAspect: { value: number }
    nextAspect: { value: number }
    viewportAspect: { value: number }
    currentRotation: { value: THREE.Matrix3 }
    nextRotation: { value: THREE.Matrix3 }
    transitionMix: { value: number }
    overlayOpacity: { value: number }
  } = {
    currentMap: { value: fallbackTexture },
    nextMap: { value: fallbackTexture },
    currentCube: { value: fallbackCubeTexture },
    nextCube: { value: fallbackCubeTexture },
    currentKind: { value: 0 },
    nextKind: { value: 0 },
    currentAspect: { value: 16 / 9 },
    nextAspect: { value: 16 / 9 },
    viewportAspect: { value: 16 / 9 },
    currentRotation: { value: new THREE.Matrix3() },
    nextRotation: { value: new THREE.Matrix3() },
    transitionMix: { value: 0 },
    overlayOpacity: { value: 0.45 },
  }
  const compositeMaterial = new THREE.ShaderMaterial({
    uniforms: compositeUniforms,
    vertexShader: `
      varying vec2 vUv;
      void main() {
        vUv = uv;
        gl_Position = vec4(position.xy, 0.0, 1.0);
      }
    `,
    fragmentShader: `
      uniform sampler2D currentMap;
      uniform sampler2D nextMap;
      uniform samplerCube currentCube;
      uniform samplerCube nextCube;
      uniform float currentKind;
      uniform float nextKind;
      uniform float currentAspect;
      uniform float nextAspect;
      uniform float viewportAspect;
      uniform mat3 currentRotation;
      uniform mat3 nextRotation;
      uniform float transitionMix;
      uniform float overlayOpacity;
      varying vec2 vUv;

      vec3 sampleImage(sampler2D map, float imageAspect) {
        vec2 uv = vUv;
        if (imageAspect > viewportAspect) {
          uv.x = (uv.x - 0.5) * (viewportAspect / imageAspect) + 0.5;
        } else {
          uv.y = (uv.y - 0.5) * (imageAspect / viewportAspect) + 0.5;
        }
        return texture2D(map, uv).rgb;
      }

      vec3 samplePanorama(samplerCube cubeMap, mat3 rotation) {
        vec2 p = vUv * 2.0 - 1.0;
        float fovScale = tan(radians(70.0) * 0.5);
        vec3 dir = normalize(vec3(p.x * viewportAspect * fovScale, p.y * fovScale, -1.0));
        return textureCube(cubeMap, rotation * dir).rgb;
      }

      vec3 sampleMedia(float kind, sampler2D map, samplerCube cubeMap, float imageAspect, mat3 rotation) {
        if (kind < 0.5) {
          return sampleImage(map, imageAspect);
        }
        return samplePanorama(cubeMap, rotation);
      }

      void main() {
        float mixAmount = clamp(transitionMix, 0.0, 1.0);
        vec3 color = sampleMedia(currentKind, currentMap, currentCube, currentAspect, currentRotation);
        if (mixAmount > 0.001) {
          vec3 nextColor = sampleMedia(nextKind, nextMap, nextCube, nextAspect, nextRotation);
          color = mix(color, nextColor, mixAmount);
        }
        color = mix(color, vec3(0.0), clamp(overlayOpacity, 0.0, 1.0));
        gl_FragColor = vec4(color, 1.0);
      }
    `,
    depthTest: false,
    depthWrite: false,
  })
  compositeScene.add(new THREE.Mesh(new THREE.PlaneGeometry(2, 2), compositeMaterial))

  let media: HomepageMedia[] = []
  const prepared = new Map<string, PreparedMedia>()
  const textureLoader = new THREE.TextureLoader()
  const cubeLoader = new THREE.CubeTextureLoader()
  const rotationEuler = new THREE.Euler(0, 0, 0, 'YXZ')
  const rotationMatrix4 = new THREE.Matrix4()

  let current = 0
  let next = 0
  let transitioning = false
  let transStart = 0
  let itemStart = performance.now()
  let rafId = 0
  let lastFrameAt = 0
  let running = false
  let listenersBound = false

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
      const entry: PreparedMedia = {
        item,
        kind: 'panorama',
        texture: cubeLoader.load(urls, () => {
          entry.ready = true
        }),
        ready: false,
      }
      entry.texture.colorSpace = THREE.SRGBColorSpace
      prepared.set(item.id, entry)
      return
    }
    const entry: PreparedMedia = {
      item,
      kind: 'image',
      texture: textureLoader.load(mediaUrl(item), () => {
        entry.ready = true
      }),
      ready: false,
    }
    entry.texture.colorSpace = THREE.SRGBColorSpace
    prepared.set(item.id, entry)
  }

  function resize() {
    if (!target || !renderer) return
    const rect = target.getBoundingClientRect()
    cssW = Math.max(Math.ceil(rect.width), 1)
    cssH = Math.max(Math.ceil(rect.height), 1)
    dpr = sceneDpr()
    renderer.setPixelRatio(dpr)
    renderer.setSize(cssW, cssH, false)
    compositeUniforms.viewportAspect.value = cssW / cssH
  }

  function renderPrepared(
    currentEntry: PreparedMedia,
    nextEntry: PreparedMedia,
    now: number,
    progress: number,
    overlay: number,
  ) {
    if (!renderer) return
    setMediaUniforms('current', currentEntry, now, itemStart)
    setMediaUniforms('next', nextEntry, now, transStart)
    compositeUniforms.transitionMix.value = progress
    compositeUniforms.overlayOpacity.value = overlay
    renderer.render(compositeScene, quadCamera)
  }

  function render(now: number) {
    if (!renderer) return
    let transitionAlpha = 0
    const candidateNext = media.length > 1 ? (current + 1) % media.length : current
    const candidateNextEntry = media[candidateNext] ? prepared.get(media[candidateNext].id) : null

    if (media.length > 0) {
      const active = media[current]
      const duration = Math.max(active?.duration_ms || 6000, 1000)
      if (
        media.length > 1 &&
        !transitioning &&
        now - itemStart >= duration &&
        candidateNextEntry?.ready
      ) {
        next = candidateNext
        transitioning = true
        transStart = now
      }
      if (transitioning) {
        transitionAlpha = easeInOut(Math.min((now - transStart) / transition, 1))
        if (transitionAlpha >= 1) {
          current = next
          transitioning = false
          itemStart = transStart
          transitionAlpha = 0
        }
      }
    }

    const currentItem = media[current]
    const currentEntry = currentItem ? prepared.get(currentItem.id) : null
    const nextItem = transitioning ? media[next] : currentItem
    const nextEntry = transitioning && nextItem ? prepared.get(nextItem.id) : currentEntry

    if (currentEntry?.ready && nextEntry?.ready) {
      const overlay = overlayOpacity(
        currentItem,
        transitioning ? nextItem : undefined,
        transitionAlpha,
      )
      renderPrepared(currentEntry, nextEntry, now, transitionAlpha, overlay)
    } else {
      renderFallback()
    }
  }

  function setMediaUniforms(
    slot: 'current' | 'next',
    entry: PreparedMedia,
    now: number,
    startedAt: number,
  ) {
    const prefix = slot === 'current' ? 'current' : 'next'
    compositeUniforms[`${prefix}Kind`].value = entry.kind === 'panorama' ? 1 : 0
    compositeUniforms[`${prefix}Aspect`].value = imageAspect(entry)

    if (entry.kind === 'panorama') {
      compositeUniforms[`${prefix}Cube`].value = entry.texture
      setPanoramaRotation(entry.item, now, startedAt, compositeUniforms[`${prefix}Rotation`].value)
    } else {
      compositeUniforms[`${prefix}Map`].value = entry.texture
    }
  }

  function imageAspect(entry: PreparedMedia) {
    if (entry.kind === 'panorama') return 16 / 9
    const image = entry.texture.image as HTMLImageElement | undefined
    const iw = image?.naturalWidth || image?.width || 16
    const ih = image?.naturalHeight || image?.height || 9
    return iw / ih
  }

  function setPanoramaRotation(
    item: HomepageMedia,
    now: number,
    startedAt: number,
    target: THREE.Matrix3,
  ) {
    const startYaw = numberConfig(item, 'start_yaw', 0)
    const startPitch = numberConfig(item, 'start_pitch', 0)
    const yawSpeed = numberConfig(item, 'yaw_speed_dps', 4)
    const pitchSpeed = numberConfig(item, 'pitch_speed_dps', 0)
    const elapsed = (now - startedAt) / 1000
    const yaw = THREE.MathUtils.degToRad(startYaw + yawSpeed * elapsed)
    const pitch = THREE.MathUtils.degToRad(startPitch + pitchSpeed * elapsed)
    rotationEuler.set(pitch, yaw, 0, 'YXZ')
    rotationMatrix4.makeRotationFromEuler(rotationEuler)
    target.setFromMatrix4(rotationMatrix4)
  }

  function renderFallback() {
    if (!renderer) return
    renderer.setClearColor(0x1a1a1a, 1)
    renderer.clear()
  }

  function overlayOpacity(currentItem?: HomepageMedia, nextItem?: HomepageMedia, progress = 0) {
    const from = currentItem ? numberConfig(currentItem, 'overlay_opacity', 0.45) : 0.45
    if (!transitioning || !nextItem) return from
    const to = numberConfig(nextItem, 'overlay_opacity', 0.45)
    return lerp(from, to, progress)
  }

  function loop() {
    if (!running) return
    const now = performance.now()
    if (now - lastFrameAt >= minFrameInterval) {
      lastFrameAt = now
      render(now)
    }
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
        renderer = new THREE.WebGLRenderer({ canvas, antialias: false, alpha: false })
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
    start() {
      if (running) return
      running = true
      bindListeners()
      itemStart = performance.now()
      lastFrameAt = 0
      rafId = requestAnimationFrame(loop)
    },
    stop() {
      running = false
      cancelAnimationFrame(rafId)
    },
    destroy() {
      this.stop()
      unbindListeners()
      for (const entry of prepared.values()) entry.texture.dispose()
      prepared.clear()
      fallbackTexture.dispose()
      fallbackCubeTexture.dispose()
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

function sceneDpr() {
  return Math.min(Math.max(window.devicePixelRatio || 1, 1), 1.5)
}

function createFallbackTexture() {
  const canvas = createSolidCanvas()
  const texture = new THREE.CanvasTexture(canvas)
  texture.colorSpace = THREE.SRGBColorSpace
  return texture
}

function createFallbackCubeTexture() {
  const faces = Array.from({ length: 6 }, () => createSolidCanvas())
  const texture = new THREE.CubeTexture(faces)
  texture.colorSpace = THREE.SRGBColorSpace
  texture.needsUpdate = true
  return texture
}

function createSolidCanvas() {
  const canvas = document.createElement('canvas')
  canvas.width = 1
  canvas.height = 1
  const ctx = canvas.getContext('2d')
  if (ctx) {
    ctx.fillStyle = '#1a1a1a'
    ctx.fillRect(0, 0, 1, 1)
  }
  return canvas
}

function lerp(a: number, b: number, t: number) {
  return a + (b - a) * t
}

function easeInOut(t: number) {
  return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2
}
