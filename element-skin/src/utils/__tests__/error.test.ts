import { describe, expect, it } from 'vitest'

import { getErrorMessage, isExternalIdentityReauthorizationRequired } from '../error'

describe('getErrorMessage', () => {
  it('prefers detail over OAuth error description', () => {
    expect(
      getErrorMessage({
        response: {
          data: {
            detail: 'permission denied',
            error_description: 'invalid refresh_token',
          },
        },
      }),
    ).toBe('permission denied')
  })

  it('uses OAuth error description when detail is absent', () => {
    expect(
      getErrorMessage({
        response: {
          data: {
            error_description: 'invalid refresh_token',
          },
        },
      }),
    ).toBe('invalid refresh_token')
  })
})

describe('isExternalIdentityReauthorizationRequired', () => {
  it('matches only the exact external identity reauthorization contract', () => {
    expect(
      isExternalIdentityReauthorizationRequired({
        response: {
          status: 409,
          data: { detail: 'external identity must be reauthorized' },
        },
      }),
    ).toBe(true)
    expect(
      isExternalIdentityReauthorizationRequired({
        response: {
          status: 502,
          data: { detail: 'external identity must be reauthorized' },
        },
      }),
    ).toBe(false)
    expect(
      isExternalIdentityReauthorizationRequired({
        response: { status: 409, data: { detail: 'another conflict' } },
      }),
    ).toBe(false)
    expect(isExternalIdentityReauthorizationRequired(new Error('network failed'))).toBe(false)
  })
})
