import type { PermissionDefinition, PermissionOverrideEffect } from '../types'

export type OAuthClientStatus = 'pending' | 'active' | 'rejected' | 'disabled'

export interface OAuthWebhookEndpoint {
  id: string
  url: string
  status: 'active' | 'disabled'
  enabled: boolean
  events: string[]
  created_at: number
  updated_at: number
  signing_secret?: string
}

export interface OAuthWebhookEndpointInput {
  id?: string
  url: string
  enabled: boolean
  events: string[]
}

export interface OAuthWebhookEventDefinition {
  type: string
  description: string
  required_permissions: string[]
  delegated_permission?: string
  application_permission?: string
}

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
  webhook_endpoints: OAuthWebhookEndpoint[]
  client_secret?: string
}

export type OAuthClientSummary = Omit<
  OAuthClient,
  'redirect_uri' | 'website_url' | 'permissions' | 'webhook_endpoints' | 'client_secret'
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
  oidc_scopes: string[]
}

export interface OAuthClientInput {
  name: string
  description?: string
  redirect_uri: string
  website_url?: string
  client_type: 'public' | 'confidential'
  permissions: string[]
  webhook_endpoints: OAuthWebhookEndpointInput[]
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

export interface OAuthPermissionScope {
  code: string
  description: string
  resource: string
  resource_description: string
  action: string
  action_description: string
  scope: string
  scope_description: string
}

export interface OAuthAuthorizationRequest {
  response_type: string
  client_id: string
  redirect_uri: string
  scope: string
  state?: string
  nonce?: string
  code_challenge: string
  code_challenge_method: string
}

export interface OAuthAuthorizationDetails {
  client: OAuthClient
  scopes: OAuthPermissionScope[]
  oidc_scopes: string[]
  redirect_uri: string
  state?: string
}

export interface OAuthAuthorizationApproval {
  code: string
  redirect_url: string
  state?: string
}

export interface DeviceAuthorizationDetails {
  client: OAuthClient
  scopes: OAuthPermissionScope[]
  expires_at: number
  status: string
}

export interface PermissionCatalogResponse {
  permissions: PermissionDefinition[]
}

export interface OAuthWebhookEventCatalogResponse {
  events: OAuthWebhookEventDefinition[]
}

export interface OpenIDConfiguration {
  issuer: string
  authorization_endpoint: string
  token_endpoint: string
  userinfo_endpoint: string
  jwks_uri: string
  revocation_endpoint: string
  response_types_supported: string[]
  grant_types_supported: string[]
  subject_types_supported: string[]
  scopes_supported: string[]
}
