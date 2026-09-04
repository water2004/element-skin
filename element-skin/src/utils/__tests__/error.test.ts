import { describe, expect, it } from 'vitest'

import {
  getApiError,
  getApiErrorMessage,
  getErrorMessage,
  isApiError,
  isExternalIdentityReauthorizationRequired,
} from '../error'

describe('getErrorMessage', () => {
  it('localizes the exact structured API error', () => {
    const error = {
      response: {
        data: {
          error: { object: 'permission', operation: 'check', reason: 'denied' },
        },
      },
    }
    expect(getApiError(error)).toEqual({
      object: 'permission',
      operation: 'check',
      reason: 'denied',
    })
    expect(isApiError(error, 'permission', 'check', 'denied')).toBe(true)
    expect(getErrorMessage(error)).toBe('没有执行此操作的权限')
  })

  it('localizes identity callback descriptors without legacy error codes', () => {
    expect(
      getApiErrorMessage({ object: 'identity', operation: 'link', reason: 'already_exists' }),
    ).toBe('该外部身份已绑定到当前账户')
    expect(getApiErrorMessage({ object: 'identity', operation: 'link', reason: 'conflict' })).toBe(
      '该外部身份已被其他账户绑定',
    )
    expect(
      getApiErrorMessage({ object: 'identity', operation: 'authorize', reason: 'incomplete' }),
    ).toBe('身份连接未完成，原有身份没有改变')
    expect(
      getApiErrorMessage({ object: 'identity', operation: 'login', reason: 'not_linked' }),
    ).toBe('该外部身份尚未绑定本站账号，请先使用账号密码登录，再到控制台的「外部身份」页进行绑定')
  })

  it('localizes OAuth protocol errors separately', () => {
    expect(
      getErrorMessage({
        response: {
          data: { error: 'invalid_grant' },
        },
      }),
    ).toBe('OAuth 授权无效或已过期')
  })

  it('uses controlled params for password requirements', () => {
    const error = {
      response: {
        data: {
          error: {
            object: 'password',
            operation: 'validate',
            reason: 'invalid',
            params: { rules: ['min_length', 'number', 'unknown'] },
          },
        },
      },
    }
    expect(getErrorMessage(error)).toBe('密码需要至少 8 个字符、包含数字')
  })

  it('rejects malformed descriptors and unknown protocol errors', () => {
    expect(getApiError({ response: { data: { error: { object: 'profile' } } } })).toBeNull()
    expect(getErrorMessage({ response: { data: { error: 'unknown_error' } } }, '失败')).toBe('失败')
  })
})

describe('isExternalIdentityReauthorizationRequired', () => {
  it('matches only the exact external identity reauthorization contract', () => {
    expect(
      isExternalIdentityReauthorizationRequired({
        response: {
          status: 409,
          data: {
            error: { object: 'identity', operation: 'authorize', reason: 'required' },
          },
        },
      }),
    ).toBe(true)
    expect(
      isExternalIdentityReauthorizationRequired({
        response: {
          status: 502,
          data: {
            error: { object: 'identity', operation: 'authorize', reason: 'required' },
          },
        },
      }),
    ).toBe(false)
    expect(
      isExternalIdentityReauthorizationRequired({
        response: {
          status: 409,
          data: { error: { object: 'identity', operation: 'authorize', reason: 'conflict' } },
        },
      }),
    ).toBe(false)
    expect(isExternalIdentityReauthorizationRequired(new Error('network failed'))).toBe(false)
  })
})
