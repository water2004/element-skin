import apiClient from '../client'
import type { OAuthClient, OAuthClientStatus, OAuthClientSummary, OAuthGrant } from './types'

export function listAdminOAuthApps(status: OAuthClientStatus | 'all' = 'all', limit = 100) {
  return apiClient.get<{ items: OAuthClientSummary[] }>('/v2/admin/oauth/apps', {
    params: { status, limit },
  })
}

export function getAdminOAuthApp(clientId: string) {
  return apiClient.get<OAuthClient>(`/v2/admin/oauth/apps/${clientId}`)
}

export function reviewAdminOAuthApp(
  clientId: string,
  status: Exclude<OAuthClientStatus, 'pending'>,
  reason = '',
) {
  return apiClient.patch<OAuthClient>(`/v2/admin/oauth/apps/${clientId}/review`, {
    status,
    reason,
  })
}

export function listAdminOAuthGrants(limit = 100) {
  return apiClient.get<{ items: OAuthGrant[] }>('/v2/admin/oauth/grants', {
    params: { limit },
  })
}

export function revokeAdminOAuthGrant(grantId: string) {
  return apiClient.delete<void>(`/v2/admin/oauth/grants/${grantId}`)
}
