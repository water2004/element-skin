import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'

interface MockAxiosInstance extends Mock {
  post: Mock
  interceptors: {
    response: {
      use: Mock
    }
  }
}

const axiosMock = vi.hoisted(() => ({
  create: vi.fn(),
  instance: undefined as unknown as MockAxiosInstance,
  onFulfilled: undefined as unknown as (response: unknown) => unknown,
  onRejected: undefined as unknown as (error: unknown) => Promise<unknown>,
}))

vi.mock('axios', () => ({
  default: {
    create: axiosMock.create,
  },
  AxiosError: class AxiosError extends Error {},
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

async function importFreshClient() {
  vi.resetModules()
  axiosMock.instance = Object.assign(vi.fn(), {
    post: vi.fn(),
    interceptors: {
      response: {
        use: vi.fn((fulfilled, rejected) => {
          axiosMock.onFulfilled = fulfilled
          axiosMock.onRejected = rejected
        }),
      },
    },
  }) as MockAxiosInstance
  axiosMock.create.mockReturnValue(axiosMock.instance)
  return import('../client')
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('api client', () => {
  it('creates the axios instance with exact base options and registers the response interceptor', async () => {
    const module = await importFreshClient()

    expect(module.default).toBe(axiosMock.instance)
    expect(axiosMock.create).toHaveBeenCalledTimes(1)
    expect(axiosMock.create).toHaveBeenCalledWith({ baseURL: '', withCredentials: true })
    expect(axiosMock.instance.interceptors.response.use).toHaveBeenCalledTimes(1)
    expect(axiosMock.instance.interceptors.response.use.mock.calls[0]).toHaveLength(2)
  })

  it('returns successful responses unchanged through the interceptor', async () => {
    await importFreshClient()
    const response = { status: 200, data: { ok: true } }

    expect(axiosMock.onFulfilled(response)).toBe(response)
  })

  it('refreshes once and retries the original request after a protected 401', async () => {
    await importFreshClient()
    const retryResponse = { status: 200, data: { ok: true } }
    axiosMock.instance.post.mockResolvedValue({ status: 200 })
    axiosMock.instance.mockResolvedValue(retryResponse)

    const original = { url: '/v1/users/me', method: 'get' }
    await expect(axiosMock.onRejected({ response: { status: 401 }, config: original })).resolves.toBe(
      retryResponse,
    )

    expect(original).toEqual({ url: '/v1/users/me', method: 'get', _retried: true })
    expect(axiosMock.instance.post).toHaveBeenCalledTimes(1)
    expect(axiosMock.instance.post).toHaveBeenCalledWith('/v1/auth/session/refresh')
    expect(axiosMock.instance).toHaveBeenCalledTimes(1)
    expect(axiosMock.instance).toHaveBeenCalledWith(original)
  })

  it('does not refresh login logout or refresh-token 401 responses', async () => {
    await importFreshClient()
    const error = { response: { status: 401 }, config: { url: '/v1/auth/login' } }

    await expect(axiosMock.onRejected(error)).rejects.toBe(error)
    expect(axiosMock.instance.post).not.toHaveBeenCalled()
    expect(axiosMock.instance).not.toHaveBeenCalled()
  })

  it('does not refresh non-401 responses or requests already retried', async () => {
    await importFreshClient()
    const forbidden = { response: { status: 403 }, config: { url: '/v1/users/me' } }
    const retried = { response: { status: 401 }, config: { url: '/v1/users/me', _retried: true } }

    await expect(axiosMock.onRejected(forbidden)).rejects.toBe(forbidden)
    await expect(axiosMock.onRejected(retried)).rejects.toBe(retried)
    expect(axiosMock.instance.post).not.toHaveBeenCalled()
    expect(axiosMock.instance).not.toHaveBeenCalled()
  })

  it('coalesces concurrent 401 refresh attempts into one refresh request', async () => {
    await importFreshClient()
    const refresh = deferred<void>()
    axiosMock.instance.post.mockReturnValue(refresh.promise)
    axiosMock.instance.mockResolvedValue({ status: 200, data: { ok: true } })

    const first = { url: '/v1/users/me', method: 'get' }
    const second = { url: '/v1/admin/users', method: 'get' }
    const firstRetry = axiosMock.onRejected({ response: { status: 401 }, config: first })
    const secondRetry = axiosMock.onRejected({ response: { status: 401 }, config: second })

    expect(axiosMock.instance.post).toHaveBeenCalledTimes(1)
    expect(axiosMock.instance.post).toHaveBeenCalledWith('/v1/auth/session/refresh')

    refresh.resolve()
    await expect(firstRetry).resolves.toEqual({ status: 200, data: { ok: true } })
    await expect(secondRetry).resolves.toEqual({ status: 200, data: { ok: true } })
    expect(axiosMock.instance).toHaveBeenCalledTimes(2)
    expect(axiosMock.instance).toHaveBeenNthCalledWith(1, first)
    expect(axiosMock.instance).toHaveBeenNthCalledWith(2, second)
  })
})
