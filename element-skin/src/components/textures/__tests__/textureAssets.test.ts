import { describe, expect, it } from 'vitest'
import {
  cacheTextureDimensions,
  probeTextureDimensions,
  textureAssetUrl,
  texturePrecision,
  type TextureImageFactory,
} from '../textureAssets'
import { texturePrecisionCases } from './fixtures/texturePrecision'

class FakeImage {
  crossOrigin: string | null = null
  onerror: (() => void) | null = null
  onload: (() => void) | null = null
  width: number
  height: number
  naturalWidth: number
  naturalHeight: number
  assignedSrc = ''
  private readonly fail: boolean

  constructor(width: number, height = width, fail = false) {
    this.width = width
    this.height = height
    this.naturalWidth = width
    this.naturalHeight = height
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

describe('probeTextureDimensions', () => {
  it('loads the image anonymously and resolves its exact natural dimensions', async () => {
    const image = new FakeImage(64)

    await expect(
      probeTextureDimensions('skin-hash', {
        baseUrl: '/',
        createImage: imageFactory([image]),
      }),
    ).resolves.toEqual({ width: 64, height: 64 })

    expect(image.crossOrigin).toBe('anonymous')
    expect(image.src).toBe('/static/textures/skin-hash.png')
  })

  it('resolves null and does not allocate an image for missing hashes', async () => {
    await expect(
      probeTextureDimensions('', {
        baseUrl: '/',
        createImage: imageFactory([]),
      }),
    ).resolves.toBeNull()
  })

  it('resolves null when the image cannot be loaded', async () => {
    const image = new FakeImage(64, 64, true)

    await expect(
      probeTextureDimensions('broken', {
        baseUrl: '/',
        createImage: imageFactory([image]),
      }),
    ).resolves.toBeNull()

    expect(image.src).toBe('/static/textures/broken.png')
  })
})

describe('texture dimensions cache', () => {
  it('returns cached dimensions without allocating another image', async () => {
    const cache = new Map([['skin-hash', { width: 128, height: 64 }]])

    await expect(
      cacheTextureDimensions('skin-hash', cache, {
        baseUrl: '/',
        createImage: imageFactory([]),
      }),
    ).resolves.toEqual({ width: 128, height: 64 })

    expect(Array.from(cache.entries())).toEqual([['skin-hash', { width: 128, height: 64 }]])
  })
})

describe('texture precision', () => {
  it.each(texturePrecisionCases)(
    '$type $width×$height has scale $scale',
    ({ type, width, height, scale }) => {
      expect(texturePrecision(type, { width, height })).toBe(scale)
    },
  )
})
