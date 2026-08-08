import { describe, expect, it } from 'vitest'
import type { OAuthWebhookEventDefinition } from '@/api/oauth'
import type { PermissionDefinition } from '@/api/types'
import {
  availableOAuthWebhookEvents,
  delegableOAuthPermissions,
  endpointsWithAllowedEvents,
  oauthAppValidationError,
  oauthClientPayload,
  permissionsForOAuthClientType,
  type OAuthAppFormState,
} from '../oauthAppFormState'

function permission(code: string, scope: PermissionDefinition['scope']): PermissionDefinition {
  return {
    id: code.length,
    code,
    description: code,
    bit_index: code.length,
    resource: code.split('.')[0],
    resource_description: code,
    action: 'read',
    action_description: '读取',
    scope,
    scope_description: scope,
  }
}

const permissionCatalog = [
  permission('profile.read.owned', 'user'),
  permission('profile.read.any', 'server'),
  permission('profile.read.system', 'system'),
]

const eventCatalog: OAuthWebhookEventDefinition[] = [
  {
    type: 'profile.updated',
    description: '角色更新',
    required_permissions: ['profile.read.any', 'profile.read.owned'],
  },
  {
    type: 'texture.updated',
    description: '材质更新',
    required_permissions: ['texture.read.owned'],
  },
]

function form(overrides: Partial<OAuthAppFormState> = {}): OAuthAppFormState {
  return {
    name: ' Example app ',
    description: ' Description ',
    redirect_uri: ' https://app.example/callback ',
    website_url: ' https://app.example ',
    client_type: 'confidential',
    permissions: ['profile.read.any'],
    webhook_endpoints: [
      {
        key: 'endpoint-1',
        id: 'wh_1',
        url: ' https://hooks.example/events ',
        enabled: true,
        events: ['profile.updated'],
      },
    ],
    ...overrides,
  }
}

describe('oauthAppFormState', () => {
  it('derives delegable permissions exactly for public and confidential clients', () => {
    expect(
      delegableOAuthPermissions(permissionCatalog, ['profile.read.owned'], 'public').map(
        (item) => item.code,
      ),
    ).toEqual(['profile.read.owned'])
    expect(
      delegableOAuthPermissions(permissionCatalog, ['profile.read.owned'], 'confidential').map(
        (item) => item.code,
      ),
    ).toEqual(['profile.read.owned', 'profile.read.any'])
  })

  it('removes server permissions when a client becomes public', () => {
    const selected = ['profile.read.owned', 'profile.read.any', 'missing.permission']
    expect(permissionsForOAuthClientType(selected, permissionCatalog, 'public')).toEqual([
      'profile.read.owned',
      'missing.permission',
    ])
    expect(permissionsForOAuthClientType(selected, permissionCatalog, 'confidential')).toEqual(
      selected,
    )
  })

  it('filters available events and configured endpoint events by selected permissions', () => {
    expect(
      availableOAuthWebhookEvents(eventCatalog, ['profile.read.owned']).map((event) => event.type),
    ).toEqual(['profile.updated'])
    expect(availableOAuthWebhookEvents(eventCatalog, ['account.read.self'])).toEqual([])
    expect(
      endpointsWithAllowedEvents(
        [
          {
            key: 'endpoint-1',
            url: 'https://hooks.example/events',
            enabled: true,
            events: ['profile.updated', 'texture.updated'],
          },
        ],
        new Set(['profile.updated']),
      ),
    ).toEqual([
      {
        key: 'endpoint-1',
        url: 'https://hooks.example/events',
        enabled: true,
        events: ['profile.updated'],
      },
    ])
  })

  it('builds an exact trimmed API payload without form-only keys', () => {
    expect(oauthClientPayload(form())).toEqual({
      name: 'Example app',
      description: 'Description',
      redirect_uri: 'https://app.example/callback',
      website_url: 'https://app.example',
      client_type: 'confidential',
      permissions: ['profile.read.any'],
      webhook_endpoints: [
        {
          id: 'wh_1',
          url: 'https://hooks.example/events',
          enabled: true,
          events: ['profile.updated'],
        },
      ],
    })
  })

  it('returns exact validation errors for every invalid form state', () => {
    expect(oauthAppValidationError(form({ name: ' ' }))).toBe('请填写应用名称')
    expect(
      oauthAppValidationError(
        form({ webhook_endpoints: [{ key: 'new-1', url: ' ', enabled: true, events: [] }] }),
      ),
    ).toBe('请填写 Webhook 接收地址')
    expect(
      oauthAppValidationError(
        form({
          webhook_endpoints: [
            { key: 'new-1', url: 'https://hooks.example/events', enabled: true, events: [] },
          ],
        }),
      ),
    ).toBe('每个 Webhook endpoint 至少选择一个监听事件')
    expect(oauthAppValidationError(form())).toBe('')
  })
})
