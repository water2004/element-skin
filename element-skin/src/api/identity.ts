import client from './client'
import type { ExternalIdentity, IdentityProvider, ItemListResponse } from './types'

export function getIdentityProviders(): Promise<{
  data: ItemListResponse<IdentityProvider> & { redirect_uri: string }
}> {
  return client.get('/v2/auth/identity-providers')
}

export function startIdentityAuthorization(data: {
  provider_id: string
  intent: 'login' | 'link'
  identity_id?: string
}): Promise<{ data: { authorization_url: string; expires_in: number } }> {
  return client.post('/v2/identity-authorizations', data)
}

export function getExternalIdentities(): Promise<{ data: ItemListResponse<ExternalIdentity> }> {
  return client.get('/v2/users/me/identities')
}

export function patchExternalIdentity(
  identityId: string,
  data: { label: string },
): Promise<{ data: void }> {
  return client.patch(`/v2/users/me/identities/${identityId}`, data)
}

export function deleteExternalIdentity(identityId: string): Promise<{ data: void }> {
  return client.delete(`/v2/users/me/identities/${identityId}`)
}
