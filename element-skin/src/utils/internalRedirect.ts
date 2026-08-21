export function internalRedirectTarget(value: unknown, fallback = '/dashboard') {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return fallback
  }
  if (value.includes('\\') || /[\u0000-\u001f\u007f]/.test(value)) {
    return fallback
  }
  try {
    const parsed = new URL(value, 'https://element-skin.internal')
    if (parsed.origin !== 'https://element-skin.internal' || parsed.pathname === '/login') {
      return fallback
    }
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return fallback
  }
}

export function loginRedirectLocation(
  current: Pick<Location, 'pathname' | 'search' | 'hash'>,
  baseUrl = '/',
) {
  const base = baseUrl.replace(/\/$/, '')
  const loginPath = `${base}/login`
  const pathname =
    base && current.pathname.startsWith(base)
      ? current.pathname.slice(base.length) || '/'
      : current.pathname
  const returnTo = internalRedirectTarget(`${pathname}${current.search}${current.hash}`, '')
  if (!returnTo) return loginPath

  return `${loginPath}?${new URLSearchParams({ redirect: returnTo }).toString()}`
}
