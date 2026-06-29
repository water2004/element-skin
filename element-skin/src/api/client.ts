import axios, { AxiosError, type AxiosRequestConfig } from 'axios'

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '',
  withCredentials: true,
})

// 不参与"401 自动刷新"的端点：刷新接口自身、登录、登出（避免死循环）。
const NO_REFRESH_PATHS = ['/v1/auth/session/refresh', '/v1/auth/login', '/v1/auth/logout']

function isNoRefreshPath(url: string | undefined): boolean {
  if (!url) return false
  const path = url.split('?')[0] ?? url
  return NO_REFRESH_PATHS.some((p) => path.endsWith(p))
}

// 需要登录的路由前缀。是否跳登录取决于**用户当前所在页面**，而非哪个接口 401——
// /v1/users/me 这类探针在公共页（首页等）也会调用并 401，按接口判断会误伤公共页访客。
const PROTECTED_PREFIXES = ['/dashboard', '/admin', '/skin-library', '/notifications']

function stripBase(pathname: string): string {
  // 去掉部署 base（如生产子目录 /skin/），使前缀判断在 dev 与 prod 一致。
  const base = (import.meta.env.BASE_URL || '/').replace(/\/$/, '')
  if (base && pathname.startsWith(base)) {
    return pathname.slice(base.length) || '/'
  }
  return pathname
}

function isOnProtectedRoute(): boolean {
  if (typeof window === 'undefined') return false
  const path = stripBase(window.location.pathname)
  return PROTECTED_PREFIXES.some((prefix) => path === prefix || path.startsWith(prefix + '/'))
}

function redirectToLogin(): void {
  if (typeof window === 'undefined') return
  const base = (import.meta.env.BASE_URL || '/').replace(/\/$/, '')
  const loginPath = base + '/login'
  if (window.location.pathname !== loginPath) {
    window.location.assign(loginPath)
  }
}

// 并发去重：同一时刻只发起一次刷新，其它 401 请求共享这枚 Promise。
let refreshPromise: Promise<void> | null = null

function runRefresh(): Promise<void> {
  if (!refreshPromise) {
    refreshPromise = apiClient
      .post('/v1/auth/session/refresh')
      .then(() => undefined)
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as (AxiosRequestConfig & { _retried?: boolean }) | undefined

    // 仅在 401、非刷新/登录端点、且未重试过时尝试一次刷新
    if (
      error.response?.status === 401 &&
      original &&
      !original._retried &&
      !isNoRefreshPath(original.url)
    ) {
      original._retried = true
      try {
        await runRefresh()
      } catch {
        // 刷新失败：仅当用户当前停留在受保护页面时才跳登录；
        // 公共页（首页等）的 401 交由调用方（如 fetchMe）静默处理。
        if (isOnProtectedRoute()) {
          redirectToLogin()
        }
        return Promise.reject(error)
      }
      // 刷新成功（新 cookie 已写入），重试原请求
      return apiClient(original)
    }

    return Promise.reject(error)
  },
)

export default apiClient
