import { describe, expect, it } from 'vitest'
import {
  applyIdentityProviderAdapterDefaults,
  emptyIdentityProviderForm,
  identityProviderPayload,
  identityProviderValidationError,
  microsoftConsumerIssuer,
} from '../identityProviderFormState'

describe('identityProviderFormState', () => {
  it('fills the canonical Microsoft consumer issuer and required scopes exactly', () => {
    const form = emptyIdentityProviderForm()
    form.adapter = 'microsoft'
    form.scopes = 'openid custom.scope XboxLive.signin'

    applyIdentityProviderAdapterDefaults(form)

    expect(form.issuer_url).toBe(microsoftConsumerIssuer)
    expect(form.scopes.split(' ')).toEqual([
      'openid',
      'custom.scope',
      'XboxLive.signin',
      'profile',
      'email',
      'offline_access',
    ])
  })

  it('preserves an explicitly configured Microsoft issuer and does nothing for generic OIDC', () => {
    const microsoft = emptyIdentityProviderForm()
    microsoft.adapter = 'microsoft'
    microsoft.issuer_url = 'https://explicit.example'
    applyIdentityProviderAdapterDefaults(microsoft)
    expect(microsoft.issuer_url).toBe('https://explicit.example')

    const generic = emptyIdentityProviderForm()
    applyIdentityProviderAdapterDefaults(generic)
    expect(generic).toEqual(emptyIdentityProviderForm())
  })

  it('builds an exact API payload without the removed registration switch', () => {
    const form = emptyIdentityProviderForm()
    Object.assign(form, {
      name: ' Microsoft ',
      issuer_url: ` ${microsoftConsumerIssuer} `,
      client_id: ' client-id ',
      client_secret: 'secret-value',
      scopes: 'openid\nprofile  XboxLive.signin offline_access',
      adapter: 'microsoft',
      icon_url: ' https://example.com/microsoft.svg ',
      enabled: true,
      login_enabled: false,
      link_enabled: true,
      display_order: 7,
    })

    expect(identityProviderPayload(form)).toEqual({
      name: 'Microsoft',
      issuer_url: microsoftConsumerIssuer,
      client_id: 'client-id',
      client_secret: 'secret-value',
      scopes: ['openid', 'profile', 'XboxLive.signin', 'offline_access'],
      adapter: 'microsoft',
      icon_url: 'https://example.com/microsoft.svg',
      enabled: true,
      login_enabled: false,
      link_enabled: true,
      display_order: 7,
    })
    expect(identityProviderPayload(form)).not.toHaveProperty('registration_enabled')
  })

  it('returns exact create and update validation errors', () => {
    const form = emptyIdentityProviderForm()
    expect(identityProviderValidationError(form, true)).toBe('请填写名称、Issuer URL 和 Client ID')

    Object.assign(form, {
      name: 'Microsoft',
      issuer_url: microsoftConsumerIssuer,
      client_id: 'client-id',
    })
    expect(identityProviderValidationError(form, true)).toBe('请填写 Client Secret')
    expect(identityProviderValidationError(form, false)).toBe('')

    form.client_secret = 'secret'
    form.scopes = ' '
    expect(identityProviderValidationError(form, true)).toBe('请填写 Scopes')
    form.scopes = 'openid'
    expect(identityProviderValidationError(form, true)).toBe('')
  })
})
