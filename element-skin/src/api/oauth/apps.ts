import apiClient from '../client'
import type { PermissionOverrideEffect } from '../types'
import type {
  OAuthClient,
  OAuthClientInput,
  OAuthClientPermissions,
  OAuthGrant,
  OAuthWebhookEventCatalogResponse,
  PermissionCatalogResponse,
} from './types'

export function listOAuthApps(limit = 50) {
  return apiClient.get<{ items: OAuthClient[] }>('/v2/oauth/apps', { params: { limit } })
}

export function getOAuthApp(clientId: string) {
  return apiClient.get<OAuthClient>(`/v2/oauth/apps/${clientId}`)
}

export function getOAuthWebhookEventCatalog() {
  return apiClient.get<OAuthWebhookEventCatalogResponse>('/v2/oauth/webhook-events')
}

export function listOAuthGrants(limit = 50) {
  return apiClient.get<{ items: OAuthGrant[] }>('/v2/oauth/grants', { params: { limit } })
}

export function revokeOAuthGrant(grantId: string) {
  return apiClient.delete<void>(`/v2/oauth/grants/${grantId}`)
}

export function getPermissionCatalog() {
  return apiClient.get<PermissionCatalogResponse>('/v2/permissions/catalog')
}

export function createOAuthApp(payload: OAuthClientInput) {
  return apiClient.post<OAuthClient>('/v2/oauth/apps', payload)
}

export function updateOAuthApp(clientId: string, payload: OAuthClientInput & { status?: string }) {
  return apiClient.patch<OAuthClient>(`/v2/oauth/apps/${clientId}`, payload)
}

export function submitOAuthAppReview(clientId: string) {
  return apiClient.post<OAuthClient>(`/v2/oauth/apps/${clientId}/review-submission`)
}

export function deleteOAuthApp(clientId: string) {
  return apiClient.delete<void>(`/v2/oauth/apps/${clientId}`)
}

export function rotateOAuthSecret(clientId: string) {
  return apiClient.post<OAuthClient>(`/v2/oauth/apps/${clientId}/secret`)
}

export function getOAuthClientPermissions(clientId: string) {
  return apiClient.get<OAuthClientPermissions>(`/v2/oauth/apps/${clientId}/permissions`)
}

export function setOAuthClientPermission(
  clientId: string,
  permissionCode: string,
  effect: PermissionOverrideEffect,
) {
  return apiClient.put<void>(`/v2/oauth/apps/${clientId}/permissions/${permissionCode}`, {
    effect,
  })
}

export function clearOAuthClientPermission(clientId: string, permissionCode: string) {
  return apiClient.delete<void>(`/v2/oauth/apps/${clientId}/permissions/${permissionCode}`)
}
