export type TextureImageFactory = () => HTMLImageElement

export interface TextureImageOptions {
  baseUrl?: string
  createImage?: TextureImageFactory
}

export interface TextureDimensions {
  width: number
  height: number
}

export function textureAssetUrl(
  hash: string | null | undefined,
  baseUrl = import.meta.env.BASE_URL,
): string {
  if (!hash) return ''
  const base = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
  return `${base}/static/textures/${hash}.png`
}

export function probeTextureDimensions(
  hash: string | null | undefined,
  options: TextureImageOptions = {},
): Promise<TextureDimensions | null> {
  const src = textureAssetUrl(hash, options.baseUrl)
  if (!src) return Promise.resolve(null)

  return new Promise((resolve) => {
    const img = options.createImage?.() ?? new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      const width = img.naturalWidth || img.width
      const height = img.naturalHeight || img.height
      resolve(width > 0 && height > 0 ? { width, height } : null)
    }
    img.onerror = () => resolve(null)
    img.src = src
  })
}

export async function cacheTextureDimensions(
  hash: string | null | undefined,
  cache: Map<string, TextureDimensions>,
  options: TextureImageOptions = {},
): Promise<TextureDimensions | null> {
  if (!hash) return null
  const cached = cache.get(hash)
  if (cached !== undefined) return cached

  const dimensions = await probeTextureDimensions(hash, options)
  if (dimensions !== null) cache.set(hash, dimensions)
  return dimensions
}

/** Pixel density, not canvas shape: a cropped 22×17 cape is also a 1× texture. */
export function texturePrecision(
  type: string,
  { width, height }: TextureDimensions,
): number | null {
  if (width <= 0 || height <= 0) return null
  if (type === 'skin' && width % 64 === 0 && (height === width || height * 2 === width)) {
    return width / 64
  }
  if (type === 'cape') {
    if (width % 64 === 0 && width === height * 2) return width / 64
    if (width % 22 === 0 && width * 17 === height * 22) return width / 22
  }
  return null
}
