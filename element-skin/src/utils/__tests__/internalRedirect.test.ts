import { describe, expect, it } from 'vitest'
import { internalRedirectTarget, loginRedirectLocation } from '../internalRedirect'

describe('internalRedirectTarget', () => {
  it.each([
    [
      '/oauth/authorize?client_id=client-1&state=opaque#consent',
      '/oauth/authorize?client_id=client-1&state=opaque#consent',
    ],
    ['/dashboard', '/dashboard'],
  ])('preserves safe internal target %s', (input, expected) => {
    expect(internalRedirectTarget(input)).toBe(expected)
  })

  it.each([
    undefined,
    'dashboard',
    '//attacker.example/callback',
    '/\\attacker.example/callback',
    '/login?redirect=/oauth/authorize',
    '/oauth/authorize\nhttps://attacker.example',
  ])('rejects unsafe target %s', (input) => {
    expect(internalRedirectTarget(input)).toBe('/dashboard')
  })

  it('uses the requested fallback exactly', () => {
    expect(internalRedirectTarget('https://attacker.example', '')).toBe('')
  })
})

describe('loginRedirectLocation', () => {
  it('preserves the complete authorization request under the deployment base', () => {
    expect(
      loginRedirectLocation(
        {
          pathname: '/skin/oauth/authorize',
          search: '?client_id=demo&state=a%2Bb&nonce=oidc-nonce',
          hash: '#consent',
        },
        '/skin/',
      ),
    ).toBe(
      '/skin/login?redirect=%2Foauth%2Fauthorize%3Fclient_id%3Ddemo%26state%3Da%252Bb%26nonce%3Doidc-nonce%23consent',
    )
  })

  it.each([
    [{ pathname: '/login', search: '', hash: '' }, '/', '/login'],
    [{ pathname: '/skin/login', search: '?redirect=/admin', hash: '' }, '/skin/', '/skin/login'],
  ])('does not create a recursive login return target', (current, base, expected) => {
    expect(loginRedirectLocation(current, base)).toBe(expected)
  })
})
