import axios, { AxiosError, type AxiosRequestConfig } from 'axios'

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '',
  withCredentials: true,
})

// 不参与"401 自动刷新"的端点：刷新接口自身、登录、登出。
// 这些接口返回 401 时直接抛出，避免死循环。
const NO_REFRESH_PATHS = ['/me/refresh-token', '/site-login', '/site-logout']

function isNoRefreshPath(url: string | undefined): boolean {
  if (!url) return false
  return NO_REFRESH_PATHS.some((p) => url.endsWith(p))
}

// 并发去重：同一时刻只发起一次刷新，其它 401 请求共享这枚 Promise。
let refreshPromise: Promise<void> | null = null

function runRefresh(): Promise<void> {
  if (!refreshPromise) {
    refreshPromise = apiClient
      .post('/me/refresh-token')
      .then(() => undefined)
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

function redirectToLogin(): void {
  if (typeof window !== 'undefined' && window.location.pathname !== '/login') {
    window.location.assign('/login')
  }
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
        redirectToLogin()
        return Promise.reject(error)
      }
      // 刷新成功（新 cookie 已写入），重试原请求
      return apiClient(original)
    }

    return Promise.reject(error)
  },
)

export default apiClient
