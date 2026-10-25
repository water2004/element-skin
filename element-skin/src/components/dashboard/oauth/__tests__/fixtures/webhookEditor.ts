import type { OAuthWebhookEventDefinition } from '@/api/oauth'
import type { WebhookEndpointForm } from '../../oauthAppFormState'

export const endpointFixtures: WebhookEndpointForm[] = [
  { key: 'first', url: 'https://first.example/events', enabled: true, events: ['profile.created'] },
  { key: 'second', url: 'https://second.example/events', enabled: false, events: [] },
]

export const eventFixtures: OAuthWebhookEventDefinition[] = [
  {
    type: 'profile.created',
    description: '角色创建',
    required_permissions: ['profile.read.owned'],
    delegated_permission: 'profile.read.owned',
  },
]
