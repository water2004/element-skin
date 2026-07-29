import type { FallbackEndpoint, FallbackRow } from '@/components/admin/mojang/types'

export interface FallbackSettingsForm {
  fallback_strategy: string
  fallback_probe_interval: number
}

export interface NormalizedFallbackSettings {
  settings: FallbackSettingsForm
  rows: FallbackRow[]
}

export const defaultFallbackSettings: FallbackSettingsForm = {
  fallback_strategy: 'serial',
  fallback_probe_interval: 600,
}

function endpointMatchesRow(endpoint: FallbackEndpoint, row: FallbackRow): boolean {
  return (
    (row.id !== null && row.id === endpoint.id) ||
    (row.session_url === endpoint.session_url && row.note === (endpoint.note || ''))
  )
}

function skinDomainsText(value: FallbackEndpoint['skin_domains']): string {
  return value.join(',')
}

function normalizeProbeInterval(value: unknown): number {
  const interval = Number(value ?? defaultFallbackSettings.fallback_probe_interval)
  return Number.isFinite(interval) && interval >= 60
    ? interval
    : defaultFallbackSettings.fallback_probe_interval
}

export function createFallbackRowFromEndpoint(
  endpoint: FallbackEndpoint,
  index: number,
  existingRows: FallbackRow[],
  now = Date.now(),
): FallbackRow {
  const existing = existingRows.find((row) => endpointMatchesRow(endpoint, row))

  return {
    ...endpoint,
    rowKey: endpoint.id ?? existing?.rowKey ?? `new_${now}_${index}`,
    note: endpoint.note || '',
    skin_domains_text: skinDomainsText(endpoint.skin_domains),
    _whitelist: existing?._whitelist ?? [],
    _initialWhitelist: existing?._initialWhitelist ?? [],
    _new_user: existing?._new_user ?? '',
    _loaded: existing?._loaded ?? false,
  }
}

export function normalizeFallbackSettings(
  data: Record<string, unknown>,
  existingRows: FallbackRow[] = [],
  now = Date.now(),
): NormalizedFallbackSettings {
  const rawFallbacks = Array.isArray(data.fallbacks) ? (data.fallbacks as FallbackEndpoint[]) : []
  const rows = rawFallbacks
    .map((endpoint, index) => createFallbackRowFromEndpoint(endpoint, index, existingRows, now))
    .sort((a, b) => a.priority - b.priority)

  return {
    settings: {
      fallback_strategy:
        (data.fallback_strategy as string) || defaultFallbackSettings.fallback_strategy,
      fallback_probe_interval: normalizeProbeInterval(data.fallback_probe_interval),
    },
    rows,
  }
}

export function createEmptyFallbackRow(index: number, now = Date.now()): FallbackRow {
  return {
    id: null,
    rowKey: `new_${now}_${index}`,
    priority: index + 1,
    session_url: '',
    account_url: '',
    services_url: '',
    cache_ttl: 60,
    enable_profile: true,
    enable_hasjoined: true,
    enable_whitelist: false,
    note: '',
    skin_domains_text: '',
    _whitelist: [],
    _initialWhitelist: [],
    _new_user: '',
    _loaded: true,
  }
}

export function syncFallbackPriorities(rows: FallbackRow[]): FallbackRow[] {
  rows.forEach((row, index) => {
    row.priority = index + 1
  })
  return rows
}

export function removeFallbackAt(rows: FallbackRow[], index: number): FallbackRow[] {
  const next = rows.slice()
  next.splice(index, 1)
  return syncFallbackPriorities(next)
}

export function moveFallback(rows: FallbackRow[], index: number, offset: -1 | 1): FallbackRow[] {
  const target = index + offset
  if (target < 0 || target >= rows.length) return rows

  const next = rows.slice()
  const current = next[index]!
  next[index] = next[target]!
  next[target] = current
  return syncFallbackPriorities(next)
}

export function toFallbackSettingsPayload(
  settings: FallbackSettingsForm,
  rows: FallbackRow[],
): Record<string, unknown> {
  return {
    fallback_strategy: settings.fallback_strategy,
    fallback_probe_interval: settings.fallback_probe_interval,
    fallbacks: rows.map((row) => ({
      id: row.id,
      priority: row.priority,
      session_url: row.session_url,
      account_url: row.account_url,
      services_url: row.services_url,
      cache_ttl: row.cache_ttl,
      enable_profile: !!row.enable_profile,
      enable_hasjoined: !!row.enable_hasjoined,
      enable_whitelist: !!row.enable_whitelist,
      note: row.note,
      skin_domains: row.skin_domains_text
        .split(',')
        .map((domain) => domain.trim())
        .filter((domain) => domain),
    })),
  }
}

export function findSavedEndpoint(
  localRow: FallbackRow,
  savedEndpoints: FallbackEndpoint[],
): FallbackEndpoint | undefined {
  return savedEndpoints.find(
    (endpoint) =>
      (localRow.id !== null && endpoint.id === localRow.id) ||
      (localRow.id === null &&
        endpoint.priority === localRow.priority &&
        endpoint.session_url === localRow.session_url &&
        endpoint.note === localRow.note),
  )
}
