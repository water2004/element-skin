import type { FallbackSaveGateway } from '@/components/admin/mojang/fallbackSave'
import type { FallbackEndpoint, FallbackRow } from '@/components/admin/mojang/types'

export type GatewayCall =
  | { method: 'saveSettings'; payload: Record<string, unknown> }
  | { method: 'loadSettings' }
  | { method: 'addWhitelist'; payload: { username: string; endpoint_id: number } }
  | { method: 'removeWhitelist'; username: string; endpointId: number }

export function createFallbackRow(overrides: Partial<FallbackRow> = {}): FallbackRow {
  return {
    id: 11,
    rowKey: 'row-11',
    priority: 1,
    session_url: 'https://one.example/session',
    account_url: 'https://one.example/account',
    services_url: 'https://one.example/services',
    cache_ttl: 60,
    enable_profile: true,
    enable_hasjoined: true,
    enable_whitelist: true,
    note: 'one',
    skin_domains_text: 'one.example,cdn.one.example',
    _whitelist: [{ username: 'Steve', created_at: 100 }],
    _initialWhitelist: [{ username: 'Steve', created_at: 100 }],
    _new_user: '',
    _loaded: true,
    ...overrides,
  }
}

export function endpointFromRow(row: FallbackRow, id: number): FallbackEndpoint {
  return {
    id,
    priority: row.priority,
    session_url: row.session_url,
    account_url: row.account_url,
    services_url: row.services_url,
    cache_ttl: row.cache_ttl,
    enable_profile: row.enable_profile,
    enable_hasjoined: row.enable_hasjoined,
    enable_whitelist: row.enable_whitelist,
    note: row.note,
    skin_domains: row.skin_domains_text
      .split(',')
      .map((domain) => domain.trim())
      .filter(Boolean),
  }
}

export function createGateway(savedSettings: Record<string, unknown>): {
  calls: GatewayCall[]
  gateway: FallbackSaveGateway
} {
  const calls: GatewayCall[] = []
  return {
    calls,
    gateway: {
      async saveSettings(payload) {
        calls.push({ method: 'saveSettings', payload })
      },
      async loadSettings() {
        calls.push({ method: 'loadSettings' })
        return savedSettings
      },
      async addWhitelist(payload) {
        calls.push({ method: 'addWhitelist', payload })
      },
      async removeWhitelist(username, endpointId) {
        calls.push({ method: 'removeWhitelist', username, endpointId })
      },
    },
  }
}
