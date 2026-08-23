import client from '../client'
import type { AdminIdentityProvider, ItemListResponse } from '../types'

export interface IdentityProviderInput {
  name: string
  issuer_url: string
  client_id: string
  client_secret?: string
  scopes: string[]
  adapter: 'generic_oidc' | 'microsoft'
  icon_url: string
  enabled: boolean
  login_enabled: boolean
  link_enabled: boolean
  display_order: number
}

export function getAdminIdentityProviders(): Promise<{
  data: ItemListResponse<AdminIdentityProvider> & { redirect_uri: string }
}> {
  return client.get('/v2/admin/identity-providers')
}

export function getAdminIdentityProvider(
  providerId: string,
): Promise<{ data: AdminIdentityProvider }> {
  return client.get(`/v2/admin/identity-providers/${providerId}`)
}

export function createIdentityProvider(
  data: IdentityProviderInput,
): Promise<{ data: AdminIdentityProvider }> {
  return client.post('/v2/admin/identity-providers', data)
}

export function updateIdentityProvider(
  providerId: string,
  data: IdentityProviderInput,
): Promise<{ data: AdminIdentityProvider }> {
  return client.put(`/v2/admin/identity-providers/${providerId}`, data)
}

export function deleteIdentityProvider(providerId: string): Promise<{ data: void }> {
  return client.delete(`/v2/admin/identity-providers/${providerId}`)
}
