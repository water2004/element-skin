import apiClient from '../client'
import type { OAuthClient, OAuthClientStatus, OAuthClientSummary } from './types'

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
