import type {
  OAuthClientInput,
  OAuthWebhookEndpointInput,
  OAuthWebhookEventDefinition,
} from '@/api/oauth'
import type { PermissionDefinition } from '@/api/types'

export interface WebhookEndpointForm extends OAuthWebhookEndpointInput {
  key: string
}

export interface OAuthAppFormState extends Omit<OAuthClientInput, 'webhook_endpoints'> {
  webhook_endpoints: WebhookEndpointForm[]
}

export function delegableOAuthPermissions(
  catalog: PermissionDefinition[],
  userPermissions: string[],
  clientType: OAuthClientInput['client_type'],
) {
  const userPermissionSet = new Set(userPermissions)
  return catalog.filter(
    (item) =>
      item.scope !== 'system' &&
      ((clientType === 'confidential' && item.scope === 'server') ||
        userPermissionSet.has(item.code)),
  )
}

export function permissionsForOAuthClientType(
  permissionCodes: string[],
  catalog: PermissionDefinition[],
  clientType: OAuthClientInput['client_type'],
) {
  if (clientType === 'confidential') return [...permissionCodes]
  const permissionByCode = new Map(catalog.map((item) => [item.code, item]))
  return permissionCodes.filter((code) => permissionByCode.get(code)?.scope !== 'server')
}

export function availableOAuthWebhookEvents(
  catalog: OAuthWebhookEventDefinition[],
  permissionCodes: string[],
  clientType: OAuthClientInput['client_type'],
) {
  const selected = new Set(permissionCodes)
  return catalog.filter(
    (event) =>
      Boolean(event.delegated_permission && selected.has(event.delegated_permission)) ||
      (clientType === 'confidential' &&
        Boolean(event.application_permission && selected.has(event.application_permission))),
  )
}

export function endpointsWithAllowedEvents(
  endpoints: WebhookEndpointForm[],
  allowedEventTypes: Set<string>,
) {
  return endpoints.map((endpoint) => ({
    ...endpoint,
    events: endpoint.events.filter((eventType) => allowedEventTypes.has(eventType)),
  }))
}

export function oauthClientPayload(form: OAuthAppFormState): OAuthClientInput {
  return {
    name: form.name.trim(),
    description: (form.description ?? '').trim(),
    redirect_uri: form.redirect_uri.trim(),
    website_url: (form.website_url ?? '').trim(),
    client_type: form.client_type,
    permissions: [...form.permissions],
    webhook_endpoints: form.webhook_endpoints.map((endpoint) => ({
      ...(endpoint.id ? { id: endpoint.id } : {}),
      url: endpoint.url.trim(),
      enabled: endpoint.enabled,
      events: [...endpoint.events],
    })),
  }
}

export function oauthAppValidationError(form: OAuthAppFormState) {
  if (!form.name.trim()) return '请填写应用名称'
  for (const endpoint of form.webhook_endpoints) {
    if (!endpoint.url.trim()) return '请填写 Webhook 接收地址'
    if (endpoint.events.length === 0) return '每个 Webhook endpoint 至少选择一个监听事件'
  }
  return ''
}
