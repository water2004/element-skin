import { describe, expect, it } from 'vitest'
import type { FallbackEndpoint, FallbackRow } from '@/components/admin/mojang/types'
import {
  createEmptyFallbackRow,
  createFallbackRowFromEndpoint,
  findSavedEndpoint,
  moveFallback,
  normalizeFallbackSettings,
  removeFallbackAt,
  syncFallbackPriorities,
  toFallbackSettingsPayload,
} from '@/components/admin/mojang/fallbackSettings'

const endpointA: FallbackEndpoint = {
  id: 11,
  priority: 2,
  session_url: 'https://session-a.example',
  account_url: 'https://account-a.example',
  services_url: 'https://services-a.example',
  cache_ttl: 120,
  enable_profile: true,
  enable_hasjoined: false,
  enable_whitelist: true,
  note: 'A',
  skin_domains: ['textures-a.example', 'cdn-a.example'],
}

const endpointB: FallbackEndpoint = {
  id: 12,
  priority: 1,
  session_url: 'https://session-b.example',
  account_url: 'https://account-b.example',
  services_url: 'https://services-b.example',
  cache_ttl: 300,
  enable_profile: false,
  enable_hasjoined: true,
  enable_whitelist: false,
  note: 'B',
  skin_domains: ['textures-b.example'],
}

const existingRow: FallbackRow = {
  id: 11,
  rowKey: 'existing-row',
  priority: 2,
  session_url: 'https://session-a.example',
  account_url: 'https://account-a.example',
  services_url: 'https://services-a.example',
  cache_ttl: 120,
  enable_profile: true,
  enable_hasjoined: false,
  enable_whitelist: true,
  note: 'A',
  skin_domains_text: 'old.example',
  _whitelist: [{ username: 'Steve', created_at: 100 }],
  _initialWhitelist: [{ username: 'Steve', created_at: 100 }],
  _new_user: 'Alex',
  _loaded: true,
}

describe('fallback settings normalization', () => {
  it('normalizes API data and preserves loaded whitelist state on matching rows', () => {
    const state = normalizeFallbackSettings(
      {
        fallback_strategy: 'parallel',
        fallback_probe_interval: '900',
        fallbacks: [endpointA, endpointB],
      },
      [existingRow],
      12345,
    )

    expect(state.settings).toEqual({
      fallback_strategy: 'parallel',
      fallback_probe_interval: 900,
    })
    expect(state.rows.map((row) => row.id)).toEqual([12, 11])
    expect(state.rows[0]).toMatchObject({
      rowKey: 12,
      skin_domains_text: 'textures-b.example',
      _loaded: false,
    })
    expect(state.rows[1]).toMatchObject({
      rowKey: 11,
      skin_domains_text: 'textures-a.example,cdn-a.example',
      _whitelist: [{ username: 'Steve', created_at: 100 }],
      _initialWhitelist: [{ username: 'Steve', created_at: 100 }],
      _new_user: 'Alex',
      _loaded: true,
    })
  })

  it('falls back to default settings and empty rows for invalid response values', () => {
    const state = normalizeFallbackSettings(
      {
        fallback_strategy: '',
        fallback_probe_interval: 30,
        fallbacks: 'not-an-array',
      },
      [],
      12345,
    )

    expect(state).toEqual({
      settings: {
        fallback_strategy: 'serial',
        fallback_probe_interval: 600,
      },
      rows: [],
    })
  })

  it('creates stable local row keys for unsaved endpoints', () => {
    const row = createFallbackRowFromEndpoint(
      { ...endpointA, id: null, skin_domains: [] },
      3,
      [],
      67890,
    )

    expect(row).toMatchObject({
      id: null,
      rowKey: 'new_67890_3',
      note: 'A',
      skin_domains_text: '',
      _loaded: false,
    })
  })
})

describe('fallback row editing helpers', () => {
  it('creates an empty enabled endpoint row with exact defaults', () => {
    expect(createEmptyFallbackRow(2, 555)).toEqual({
      id: null,
      rowKey: 'new_555_2',
      priority: 3,
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
    })
  })

  it('removes rows and rewrites priorities exactly', () => {
    const rows = [
      { ...existingRow, id: 1, priority: 1 },
      { ...existingRow, id: 2, priority: 2 },
      { ...existingRow, id: 3, priority: 3 },
    ]

    expect(removeFallbackAt(rows, 1).map((row) => ({ id: row.id, priority: row.priority }))).toEqual(
      [
        { id: 1, priority: 1 },
        { id: 3, priority: 2 },
      ],
    )
  })

  it('moves rows within bounds and leaves out-of-bounds moves unchanged', () => {
    const rows = [
      { ...existingRow, id: 1, priority: 1 },
      { ...existingRow, id: 2, priority: 2 },
      { ...existingRow, id: 3, priority: 3 },
    ]

    expect(moveFallback(rows, 2, -1).map((row) => ({ id: row.id, priority: row.priority }))).toEqual(
      [
        { id: 1, priority: 1 },
        { id: 3, priority: 2 },
        { id: 2, priority: 3 },
      ],
    )
    expect(moveFallback(rows, 0, -1)).toBe(rows)
    expect(moveFallback(rows, 2, 1)).toBe(rows)
  })

  it('syncs priorities in place and returns the same list', () => {
    const rows = [
      { ...existingRow, id: 7, priority: 9 },
      { ...existingRow, id: 8, priority: 4 },
    ]

    expect(syncFallbackPriorities(rows)).toBe(rows)
    expect(rows.map((row) => ({ id: row.id, priority: row.priority }))).toEqual([
      { id: 7, priority: 1 },
      { id: 8, priority: 2 },
    ])
  })
})

describe('fallback settings payload', () => {
  it('serializes settings and trims skin domains exactly', () => {
    expect(
      toFallbackSettingsPayload(
        { fallback_strategy: 'parallel', fallback_probe_interval: 1200 },
        [{ ...existingRow, skin_domains_text: ' textures.example, ,cdn.example ' }],
      ),
    ).toEqual({
      fallback_strategy: 'parallel',
      fallback_probe_interval: 1200,
      fallbacks: [
        {
          id: 11,
          priority: 2,
          session_url: 'https://session-a.example',
          account_url: 'https://account-a.example',
          services_url: 'https://services-a.example',
          cache_ttl: 120,
          enable_profile: true,
          enable_hasjoined: false,
          enable_whitelist: true,
          note: 'A',
          skin_domains: ['textures.example', 'cdn.example'],
        },
      ],
    })
  })

  it('finds existing endpoints by stable ID and new endpoints by session URL and note', () => {
    expect(findSavedEndpoint(existingRow, [endpointB, endpointA])).toEqual(endpointA)
    expect(findSavedEndpoint({ ...existingRow, session_url: 'changed', note: 'changed' }, [endpointA])).toEqual(
      endpointA,
    )
    expect(
      findSavedEndpoint({ ...existingRow, id: null, rowKey: 'new', note: 'missing' }, [endpointA]),
    ).toBeUndefined()
    expect(
      findSavedEndpoint({ ...existingRow, id: null, rowKey: 'new' }, [endpointB, endpointA]),
    ).toEqual(endpointA)
  })
})
