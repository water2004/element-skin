import { describe, expect, it } from 'vitest'
import {
  cacheTextureWidth,
  probeTextureWidth,
  textureAssetUrl,
  type TextureImageFactory,
} from '../textureAssets'

class FakeImage {
  crossOrigin: string | null = null
  onerror: (() => void) | null = null
  onload: (() => void) | null = null
  width: number
  assignedSrc = ''
  private readonly fail: boolean

  constructor(width: number, fail = false) {
    this.width = width
    this.fail = fail
  }

  set src(value: string) {
    this.assignedSrc = value
    queueMicrotask(() => {
      if (this.fail) {
        this.onerror?.()
        return
      }
      this.onload?.()
    })
  }

  get src() {
    return this.assignedSrc
  }
}

function imageFactory(images: FakeImage[]): TextureImageFactory {
  return () => {
    const image = images.shift()
    if (!image) throw new Error('unexpected image allocation')
    return image as unknown as HTMLImageElement
  }
}

describe('textureAssetUrl', () => {
  it('builds the static texture path with a configured base path', () => {
    expect(textureAssetUrl('abc123', '/skin/')).toBe('/skin/static/textures/abc123.png')
    expect(textureAssetUrl('abc123', '/skin')).toBe('/skin/static/textures/abc123.png')
    expect(textureAssetUrl('abc123', 'https://cdn.example/skin/')).toBe(
      'https://cdn.example/skin/static/textures/abc123.png',
    )
  })

  it('returns an empty path when the texture hash is missing', () => {
    expect(textureAssetUrl(null, '/skin/')).toBe('')
    expect(textureAssetUrl(undefined, '/skin/')).toBe('')
    expect(textureAssetUrl('', '/skin/')).toBe('')
  })
})

describe('probeTextureWidth', () => {
  it('loads the image anonymously and resolves its exact width', async () => {
    const image = new FakeImage(64)

    await expect(
      probeTextureWidth('skin-hash', {
        baseUrl: '/',
        createImage: imageFactory([image]),
      }),
    ).resolves.toBe(64)

    expect(image.crossOrigin).toBe('anonymous')
    expect(image.src).toBe('/static/textures/skin-hash.png')
  })

  it('resolves null and does not allocate an image for missing hashes', async () => {
    await expect(
      probeTextureWidth('', {
        baseUrl: '/',
        createImage: imageFactory([]),
      }),
    ).resolves.toBeNull()
  })

  it('resolves null when the image cannot be loaded', async () => {
    const image = new FakeImage(64, true)

    await expect(
      probeTextureWidth('broken', {
        baseUrl: '/',
        createImage: imageFactory([image]),
      }),
    ).resolves.toBeNull()

    expect(image.src).toBe('/static/textures/broken.png')
  })
})

describe('texture width cache', () => {
  it('returns a cached width without allocating another image', async () => {
    const cache = new Map([['skin-hash', 128]])

    await expect(
      cacheTextureWidth('skin-hash', cache, {
        baseUrl: '/',
        createImage: imageFactory([]),
      }),
    ).resolves.toBe(128)

    expect(Array.from(cache.entries())).toEqual([['skin-hash', 128]])
  })
})
