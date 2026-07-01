import apiClient from './client'
import type { PermissionDefinition, PermissionOverrideEffect } from './types'

export type OAuthClientStatus = 'pending' | 'active' | 'rejected' | 'disabled'

export interface OAuthClient {
  client_id: string
  owner_user_id: string
  name: string
  description: string
  redirect_uri: string
  website_url: string
  client_type: 'public' | 'confidential'
  status: OAuthClientStatus
  created_at: number
  updated_at: number
  permissions: string[]
  client_secret?: string
}

export type OAuthClientSummary = Omit<
  OAuthClient,
  'redirect_uri' | 'website_url' | 'permissions' | 'client_secret'
>

export interface OAuthGrant {
  id: string
  user_id: string
  subject_id: string
  client_id: string
  status: 'active' | 'revoked'
  created_at: number
  revoked_at?: number | null
  permissions: string[]
}

export interface OAuthClientInput {
  name: string
  description?: string
  redirect_uri: string
  website_url?: string
  client_type: 'public' | 'confidential'
  permissions: string[]
}

export interface OAuthClientPermissions {
  subject_id: string
  client: OAuthClient
  effective_permissions: string[]
  overrides: Array<{
    permission_code: string
    effect: PermissionOverrideEffect
    created_at: number
  }>
  client_allowed_scopes: string[]
  session_allowed_scopes: string[]
}

export interface DeviceAuthorizationDetails {
  client: OAuthClient
  scopes: PermissionDefinition[]
  expires_at: number
  status: string
}

export interface PermissionCatalogResponse {
  permissions: PermissionDefinition[]
}

export function listOAuthApps(limit = 50) {
  return apiClient.get<{ items: OAuthClient[] }>('/v1/oauth/apps', { params: { limit } })
}

export function listOAuthGrants(limit = 50) {
  return apiClient.get<{ items: OAuthGrant[] }>('/v1/oauth/grants', { params: { limit } })
}

export function revokeOAuthGrant(grantId: string) {
  return apiClient.delete<{ ok: true }>(`/v1/oauth/grants/${grantId}`)
}

export function getPermissionCatalog() {
  return apiClient.get<PermissionCatalogResponse>('/v1/permissions/catalog')
}

export function createOAuthApp(payload: OAuthClientInput) {
  return apiClient.post<OAuthClient>('/v1/oauth/apps', payload)
}

export function updateOAuthApp(clientId: string, payload: OAuthClientInput & { status?: string }) {
  return apiClient.patch<OAuthClient>(`/v1/oauth/apps/${clientId}`, payload)
}

export function submitOAuthAppReview(clientId: string) {
  return apiClient.post<OAuthClient>(`/v1/oauth/apps/${clientId}/review-submission`)
}

export function deleteOAuthApp(clientId: string) {
  return apiClient.delete<{ ok: true }>(`/v1/oauth/apps/${clientId}`)
}

export function rotateOAuthSecret(clientId: string) {
  return apiClient.post<OAuthClient>(`/v1/oauth/apps/${clientId}/secret`)
}

export function getOAuthClientPermissions(clientId: string) {
  return apiClient.get<OAuthClientPermissions>(`/v1/oauth/apps/${clientId}/permissions`)
}

export function setOAuthClientPermission(
  clientId: string,
  permissionCode: string,
  effect: PermissionOverrideEffect,
) {
  return apiClient.put<{ ok: true }>(`/v1/oauth/apps/${clientId}/permissions/${permissionCode}`, {
    effect,
  })
}

export function clearOAuthClientPermission(clientId: string, permissionCode: string) {
  return apiClient.delete<{ ok: true }>(`/v1/oauth/apps/${clientId}/permissions/${permissionCode}`)
}

export function listAdminOAuthApps(status: OAuthClientStatus | 'all' = 'all', limit = 100) {
  return apiClient.get<{ items: OAuthClientSummary[] }>('/v1/admin/oauth/apps', {
    params: { status, limit },
  })
}

export function getAdminOAuthApp(clientId: string) {
  return apiClient.get<OAuthClient>(`/v1/admin/oauth/apps/${clientId}`)
}

export function reviewAdminOAuthApp(
  clientId: string,
  status: Exclude<OAuthClientStatus, 'pending'>,
  reason = '',
) {
  return apiClient.patch<OAuthClient>(`/v1/admin/oauth/apps/${clientId}/review`, {
    status,
    reason,
  })
}

export function getDeviceAuthorization(userCode: string) {
  return apiClient.get<DeviceAuthorizationDetails>('/oauth/device', {
    params: { user_code: userCode },
  })
}

export function decideDeviceAuthorization(userCode: string, approve: boolean) {
  return apiClient.post<{ ok: true }>('/oauth/device', {
    user_code: userCode,
    approve,
  })
}
