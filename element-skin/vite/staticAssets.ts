import path from 'node:path'

export type StaticAssetRequest = {
  contentType: 'image/jpeg' | 'image/png'
  filePath: string
  rootPath: string
}

export function resolveStaticAssetRequest(
  backendRoot: string,
  base: string,
  requestUrl: string,
): StaticAssetRequest | null {
  let pathname: string
  try {
    pathname = decodeURIComponent(new URL(requestUrl, 'http://localhost').pathname)
  } catch {
    return null
  }

  const normalizedBase = normalizeBase(base)
  if (normalizedBase !== '/') {
    if (!pathname.startsWith(`${normalizedBase}/`)) return null
    pathname = pathname.slice(normalizedBase.length)
  }

  const match = pathname.match(/^\/static\/(textures|carousel)\/(.+)$/)
  if (!match) return null

  const type = match[1] as 'textures' | 'carousel'
  const requestedName = match[2]
  if (requestedName.includes('\\')) {
    return null
  }
  const rootPath = path.resolve(backendRoot, type)
  const filePath = path.resolve(rootPath, requestedName)
  if (!isPathInside(rootPath, filePath)) return null

  return {
    contentType: type === 'textures' ? 'image/png' : 'image/jpeg',
    filePath,
    rootPath,
  }
}

export function isPathInside(rootPath: string, candidatePath: string): boolean {
  const relative = path.relative(rootPath, candidatePath)
  return (
    relative !== '' &&
    !relative.startsWith(`..${path.sep}`) &&
    relative !== '..' &&
    !path.isAbsolute(relative)
  )
}

function normalizeBase(base: string): string {
  const withLeadingSlash = base.startsWith('/') ? base : `/${base}`
  if (withLeadingSlash === '/') return withLeadingSlash
  return withLeadingSlash.replace(/\/+$/, '')
}
