import type { IdentityProviderInput } from '@/api/admin/identity-providers'

export const microsoftConsumerIssuer =
  'https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0'

export interface IdentityProviderFormState {
  name: string
  issuer_url: string
  client_id: string
  client_secret: string
  scopes: string
  adapter: 'generic_oidc' | 'microsoft'
  icon_url: string
  enabled: boolean
  login_enabled: boolean
  link_enabled: boolean
  display_order: number
}

export function emptyIdentityProviderForm(): IdentityProviderFormState {
  return {
    name: '',
    issuer_url: '',
    client_id: '',
    client_secret: '',
    scopes: 'openid profile email',
    adapter: 'generic_oidc',
    icon_url: '',
    enabled: true,
    login_enabled: true,
    link_enabled: true,
    display_order: 0,
  }
}

export function applyIdentityProviderAdapterDefaults(form: IdentityProviderFormState) {
  if (form.adapter !== 'microsoft') return
  if (!form.issuer_url.trim()) form.issuer_url = microsoftConsumerIssuer
  const scopes = new Set(form.scopes.split(/\s+/).filter(Boolean))
  for (const scope of ['openid', 'profile', 'email', 'XboxLive.signin', 'offline_access']) {
    scopes.add(scope)
  }
  form.scopes = [...scopes].join(' ')
}

export function identityProviderPayload(form: IdentityProviderFormState): IdentityProviderInput {
  const result: IdentityProviderInput = {
    name: form.name.trim(),
    issuer_url: form.issuer_url.trim(),
    client_id: form.client_id.trim(),
    scopes: form.scopes.split(/\s+/).filter(Boolean),
    adapter: form.adapter,
    icon_url: form.icon_url.trim(),
    enabled: form.enabled,
    login_enabled: form.login_enabled,
    link_enabled: form.link_enabled,
    display_order: form.display_order,
  }
  if (form.client_secret) result.client_secret = form.client_secret
  return result
}

export function identityProviderValidationError(
  form: IdentityProviderFormState,
  isCreate: boolean,
) {
  if (!form.name.trim() || !form.issuer_url.trim() || !form.client_id.trim()) {
    return '请填写名称、Issuer URL 和 Client ID'
  }
  if (isCreate && !form.client_secret.trim()) return '请填写 Client Secret'
  if (!form.scopes.trim()) return '请填写 Scopes'
  return ''
}
