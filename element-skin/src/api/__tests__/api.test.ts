import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'

import client from '../client'
import { createApiCases, type ApiCase, type ApiMethod } from './fixtures'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

const mockedClient = client as unknown as Record<ApiMethod, Mock>
const methods: ApiMethod[] = ['get', 'post', 'patch', 'delete']
const mockedResponse = { data: { marker: 'api-response' } }

beforeEach(() => {
  for (const method of methods) {
    mockedClient[method].mockReset()
    mockedClient[method].mockResolvedValue(mockedResponse)
  }
})

async function expectExactRequest(testCase: ApiCase): Promise<void> {
  const result = await testCase.call()
  expect(result).toBe(mockedResponse)

  for (const method of methods) {
    if (method === testCase.method) {
      expect(mockedClient[method]).toHaveBeenCalledTimes(1)
      expect(mockedClient[method]).toHaveBeenCalledWith(...testCase.args)
    } else {
      expect(mockedClient[method]).not.toHaveBeenCalled()
    }
  }
}

describe('frontend API wrappers', () => {
  it.each(createApiCases())('$name', async (testCase) => {
    await expectExactRequest(testCase)
  })
})
