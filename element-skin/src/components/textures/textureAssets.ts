export type TextureImageFactory = () => HTMLImageElement

export interface TextureWidthOptions {
  baseUrl?: string
  createImage?: TextureImageFactory
}

export function textureAssetUrl(
  hash: string | null | undefined,
  baseUrl = import.meta.env.BASE_URL,
): string {
  if (!hash) return ''
  const base = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
  return `${base}/static/textures/${hash}.png`
}

export function probeTextureWidth(
  hash: string | null | undefined,
  options: TextureWidthOptions = {},
): Promise<number | null> {
  const src = textureAssetUrl(hash, options.baseUrl)
  if (!src) return Promise.resolve(null)

  return new Promise((resolve) => {
    const img = options.createImage?.() ?? new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => resolve(img.width)
    img.onerror = () => resolve(null)
    img.src = src
  })
}

export async function cacheTextureWidth(
  hash: string | null | undefined,
  cache: Map<string, number>,
  options: TextureWidthOptions = {},
): Promise<number | null> {
  if (!hash) return null
  const cached = cache.get(hash)
  if (cached !== undefined) return cached

  const width = await probeTextureWidth(hash, options)
  if (width !== null) cache.set(hash, width)
  return width
}
