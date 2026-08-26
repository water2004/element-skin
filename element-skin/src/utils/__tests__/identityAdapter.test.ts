import { describe, expect, it } from 'vitest'
import { identityProviderAdapterLabel } from '../identityAdapter'

describe('identityProviderAdapterLabel', () => {
  it('maps every known adapter to its dedicated display label', () => {
    expect(identityProviderAdapterLabel('generic_oidc')).toBe('通用 OIDC')
    expect(identityProviderAdapterLabel('microsoft')).toBe('Microsoft')
    expect(identityProviderAdapterLabel('qq')).toBe('QQ 互联')
  })

  it('falls back to the raw value for unknown adapters instead of a wrong family label', () => {
    expect(identityProviderAdapterLabel('wechat')).toBe('wechat')
    expect(identityProviderAdapterLabel('')).toBe('')
  })
})
