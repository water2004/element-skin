import { describe, expect, it } from 'vitest'
import {
  saveFallbackConfiguration,
  type FallbackSaveGateway,
} from '@/components/admin/mojang/fallbackSave'
import {
  createFallbackRow as createRow,
  createGateway,
  endpointFromRow,
  type GatewayCall,
} from '@/components/admin/mojang/__tests__/fixtures/fallbackSaveFixtures'

const schedulingCases = [
  { strategy: 'serial', interval: 60 },
  { strategy: 'parallel', interval: 86400 },
]

const capabilityCases = [true, false].flatMap((enableProfile) =>
  [true, false].flatMap((enableHasJoined) =>
    [true, false].map((enableWhitelist) => ({
      enableProfile,
      enableHasJoined,
      enableWhitelist,
    })),
  ),
)

const schedulingCapabilityCases = schedulingCases.flatMap((scheduling) =>
  capabilityCases.map((capabilities) => ({ ...scheduling, ...capabilities })),
)

describe('fallback configuration save orchestration', () => {
  it.each(schedulingCapabilityCases)(
    'sends every field for $strategy scheduling and profile=$enableProfile hasjoined=$enableHasJoined whitelist=$enableWhitelist',
    async ({
      strategy,
      interval,
      enableProfile,
      enableHasJoined,
      enableWhitelist,
    }) => {
      const row = createRow({
        id: 81,
        rowKey: 'row-81',
        priority: 7,
        session_url: 'https://matrix.example/session-v2',
        account_url: 'https://matrix.example/account-v2',
        services_url: 'https://matrix.example/services-v2',
        cache_ttl: 987,
        enable_profile: enableProfile,
        enable_hasjoined: enableHasJoined,
        enable_whitelist: enableWhitelist,
        note: 'matrix updated',
        skin_domains_text: ' matrix.example, ,cdn.matrix.example ',
        _whitelist: [],
        _initialWhitelist: [],
      })
      const savedSettings = {
        fallback_strategy: strategy,
        fallback_probe_interval: interval,
        fallbacks: [endpointFromRow(row, 81)],
      }
      const { calls, gateway } = createGateway(savedSettings)

      const result = await saveFallbackConfiguration(
        { fallback_strategy: strategy, fallback_probe_interval: interval },
        [row],
        gateway,
      )

      expect(result).toEqual(savedSettings)
      expect(calls).toEqual([
        {
          method: 'saveSettings',
          payload: {
            fallback_strategy: strategy,
            fallback_probe_interval: interval,
            fallbacks: [
              {
                id: 81,
                priority: 7,
                session_url: 'https://matrix.example/session-v2',
                account_url: 'https://matrix.example/account-v2',
                services_url: 'https://matrix.example/services-v2',
                cache_ttl: 987,
                enable_profile: enableProfile,
                enable_hasjoined: enableHasJoined,
                enable_whitelist: enableWhitelist,
                note: 'matrix updated',
                skin_domains: ['matrix.example', 'cdn.matrix.example'],
              },
            ],
          },
        },
        { method: 'loadSettings' },
      ])
      expect(row.id).toBe(81)
      expect(row._initialWhitelist).toEqual([])
    },
  )

  it.each(schedulingCases)(
    'sends exact endpoint reorder, update, removal, and creation packet for $strategy scheduling',
    async ({ strategy, interval }) => {
      const moved = createRow({
        id: 13,
        rowKey: 'row-13',
        priority: 1,
        session_url: 'https://three.example/session-v2',
        account_url: 'https://three.example/account-v2',
        services_url: 'https://three.example/services-v2',
        cache_ttl: 360,
        enable_profile: false,
        enable_hasjoined: false,
        enable_whitelist: true,
        note: 'three moved and updated',
        skin_domains_text: 'three-v2.example,cdn.three.example',
        _whitelist: [],
        _initialWhitelist: [],
      })
      const created = createRow({
        id: null,
        rowKey: 'new-2',
        priority: 2,
        session_url: 'https://new.example/session',
        account_url: 'https://new.example/account',
        services_url: 'https://new.example/services',
        cache_ttl: 45,
        enable_profile: true,
        enable_hasjoined: false,
        enable_whitelist: false,
        note: 'new',
        skin_domains_text: '',
        _whitelist: [],
        _initialWhitelist: [],
      })
      const retained = createRow({
        id: 11,
        rowKey: 'row-11',
        priority: 3,
        _whitelist: [],
        _initialWhitelist: [],
      })
      const rows = [moved, created, retained]
      const savedSettings = {
        fallback_strategy: strategy,
        fallback_probe_interval: interval,
        fallbacks: [
          endpointFromRow(moved, 13),
          endpointFromRow(created, 52),
          endpointFromRow(retained, 11),
        ],
      }
      const { calls, gateway } = createGateway(savedSettings)

      await saveFallbackConfiguration(
        { fallback_strategy: strategy, fallback_probe_interval: interval },
        rows,
        gateway,
      )

      expect(calls).toEqual([
        {
          method: 'saveSettings',
          payload: {
            fallback_strategy: strategy,
            fallback_probe_interval: interval,
            fallbacks: [
              {
                id: 13,
                priority: 1,
                session_url: 'https://three.example/session-v2',
                account_url: 'https://three.example/account-v2',
                services_url: 'https://three.example/services-v2',
                cache_ttl: 360,
                enable_profile: false,
                enable_hasjoined: false,
                enable_whitelist: true,
                note: 'three moved and updated',
                skin_domains: ['three-v2.example', 'cdn.three.example'],
              },
              {
                id: null,
                priority: 2,
                session_url: 'https://new.example/session',
                account_url: 'https://new.example/account',
                services_url: 'https://new.example/services',
                cache_ttl: 45,
                enable_profile: true,
                enable_hasjoined: false,
                enable_whitelist: false,
                note: 'new',
                skin_domains: [],
              },
              {
                id: 11,
                priority: 3,
                session_url: 'https://one.example/session',
                account_url: 'https://one.example/account',
                services_url: 'https://one.example/services',
                cache_ttl: 60,
                enable_profile: true,
                enable_hasjoined: true,
                enable_whitelist: true,
                note: 'one',
                skin_domains: ['one.example', 'cdn.one.example'],
              },
            ],
          },
        },
        { method: 'loadSettings' },
      ])
      expect(rows.map((row) => row.id)).toEqual([13, 52, 11])
    },
  )

  it.each(schedulingCases)(
    'sends an explicit empty endpoint list without changing $strategy scheduling fields',
    async ({ strategy, interval }) => {
      const savedSettings = {
        fallback_strategy: strategy,
        fallback_probe_interval: interval,
        fallbacks: [],
      }
      const { calls, gateway } = createGateway(savedSettings)

      const result = await saveFallbackConfiguration(
        { fallback_strategy: strategy, fallback_probe_interval: interval },
        [],
        gateway,
      )

      expect(result).toEqual(savedSettings)
      expect(calls).toEqual([
        {
          method: 'saveSettings',
          payload: {
            fallback_strategy: strategy,
            fallback_probe_interval: interval,
            fallbacks: [],
          },
        },
        { method: 'loadSettings' },
      ])
    },
  )

  it('sends exact settings and per-endpoint whitelist differences for mixed rows', async () => {
    const unchanged = createRow()
    const modified = createRow({
      id: 12,
      rowKey: 'row-12',
      priority: 2,
      session_url: 'https://two.example/session-v2',
      account_url: 'https://two.example/account-v2',
      services_url: 'https://two.example/services-v2',
      cache_ttl: 180,
      enable_profile: false,
      enable_hasjoined: true,
      enable_whitelist: true,
      note: 'two updated',
      skin_domains_text: ' two.example, ,cdn.two.example ',
      _initialWhitelist: [
        { username: 'Alex', created_at: 200 },
        { username: 'OldPlayer', created_at: 201 },
      ],
      _whitelist: [
        { username: 'alex', created_at: 202 },
        { username: 'NewPlayer', created_at: 203 },
      ],
    })
    const unloaded = createRow({
      id: 13,
      rowKey: 'row-13',
      priority: 3,
      session_url: 'https://three.example/session',
      account_url: 'https://three.example/account',
      services_url: 'https://three.example/services',
      cache_ttl: 300,
      enable_profile: true,
      enable_hasjoined: false,
      enable_whitelist: false,
      note: 'three',
      skin_domains_text: '',
      _initialWhitelist: [{ username: 'ServerState', created_at: 300 }],
      _whitelist: [{ username: 'UnsavedLocalState', created_at: 301 }],
      _loaded: false,
    })
    const created = createRow({
      id: null,
      rowKey: 'new-4',
      priority: 4,
      session_url: 'https://four.example/session',
      account_url: 'https://four.example/account',
      services_url: 'https://four.example/services',
      cache_ttl: 90,
      enable_profile: false,
      enable_hasjoined: false,
      enable_whitelist: true,
      note: 'four',
      skin_domains_text: 'four.example',
      _initialWhitelist: [],
      _whitelist: [{ username: 'CreatedPlayer', created_at: 400 }],
    })
    const rows = [unchanged, modified, unloaded, created]
    const savedSettings = {
      fallback_strategy: 'parallel',
      fallback_probe_interval: 1200,
      fallbacks: [
        endpointFromRow(unchanged, 11),
        endpointFromRow(modified, 12),
        endpointFromRow(unloaded, 13),
        endpointFromRow(created, 44),
      ],
    }
    const { calls, gateway } = createGateway(savedSettings)

    const result = await saveFallbackConfiguration(
      { fallback_strategy: 'parallel', fallback_probe_interval: 1200 },
      rows,
      gateway,
    )

    expect(result).toEqual(savedSettings)
    expect(calls).toEqual([
      {
        method: 'saveSettings',
        payload: {
          fallback_strategy: 'parallel',
          fallback_probe_interval: 1200,
          fallbacks: [
            {
              id: 11,
              priority: 1,
              session_url: 'https://one.example/session',
              account_url: 'https://one.example/account',
              services_url: 'https://one.example/services',
              cache_ttl: 60,
              enable_profile: true,
              enable_hasjoined: true,
              enable_whitelist: true,
              note: 'one',
              skin_domains: ['one.example', 'cdn.one.example'],
            },
            {
              id: 12,
              priority: 2,
              session_url: 'https://two.example/session-v2',
              account_url: 'https://two.example/account-v2',
              services_url: 'https://two.example/services-v2',
              cache_ttl: 180,
              enable_profile: false,
              enable_hasjoined: true,
              enable_whitelist: true,
              note: 'two updated',
              skin_domains: ['two.example', 'cdn.two.example'],
            },
            {
              id: 13,
              priority: 3,
              session_url: 'https://three.example/session',
              account_url: 'https://three.example/account',
              services_url: 'https://three.example/services',
              cache_ttl: 300,
              enable_profile: true,
              enable_hasjoined: false,
              enable_whitelist: false,
              note: 'three',
              skin_domains: [],
            },
            {
              id: null,
              priority: 4,
              session_url: 'https://four.example/session',
              account_url: 'https://four.example/account',
              services_url: 'https://four.example/services',
              cache_ttl: 90,
              enable_profile: false,
              enable_hasjoined: false,
              enable_whitelist: true,
              note: 'four',
              skin_domains: ['four.example'],
            },
          ],
        },
      },
      { method: 'loadSettings' },
      { method: 'addWhitelist', payload: { username: 'NewPlayer', endpoint_id: 12 } },
      { method: 'removeWhitelist', username: 'OldPlayer', endpointId: 12 },
      { method: 'addWhitelist', payload: { username: 'CreatedPlayer', endpoint_id: 44 } },
    ])
    expect(rows.map((row) => row.id)).toEqual([11, 12, 13, 44])
    expect(unchanged._initialWhitelist).toEqual([{ username: 'Steve', created_at: 100 }])
    expect(unchanged._initialWhitelist).not.toBe(unchanged._whitelist)
    expect(modified._initialWhitelist).toEqual([
      { username: 'alex', created_at: 202 },
      { username: 'NewPlayer', created_at: 203 },
    ])
    expect(modified._initialWhitelist).not.toBe(modified._whitelist)
    expect(unloaded._initialWhitelist).toEqual([{ username: 'ServerState', created_at: 300 }])
    expect(created._initialWhitelist).toEqual([{ username: 'CreatedPlayer', created_at: 400 }])
  })

  it('maps otherwise identical new endpoints by priority before sending whitelist packets', async () => {
    const first = createRow({ id: null, rowKey: 'new-1', priority: 1, _initialWhitelist: [], _whitelist: [{ username: 'First', created_at: 1 }] })
    const second = createRow({ id: null, rowKey: 'new-2', priority: 2, _initialWhitelist: [], _whitelist: [{ username: 'Second', created_at: 2 }] })
    const savedSettings = {
      fallbacks: [endpointFromRow(first, 51), endpointFromRow(second, 52)],
    }
    const { calls, gateway } = createGateway(savedSettings)

    await saveFallbackConfiguration(
      { fallback_strategy: 'serial', fallback_probe_interval: 600 },
      [first, second],
      gateway,
    )

    expect(first.id).toBe(51)
    expect(second.id).toBe(52)
    expect(calls.slice(2)).toEqual([
      { method: 'addWhitelist', payload: { username: 'First', endpoint_id: 51 } },
      { method: 'addWhitelist', payload: { username: 'Second', endpoint_id: 52 } },
    ])
  })

  it.each([
    {
      name: 'unchanged entries',
      id: 11,
      loaded: true,
      initial: [
        { username: 'Alex', created_at: 1 },
        { username: 'Steve', created_at: 2 },
      ],
      current: [
        { username: 'Alex', created_at: 1 },
        { username: 'Steve', created_at: 2 },
      ],
      expectedWhitelistCalls: [],
    },
    {
      name: 'case and order only changes',
      id: 11,
      loaded: true,
      initial: [
        { username: 'Alex', created_at: 1 },
        { username: 'Steve', created_at: 2 },
      ],
      current: [
        { username: 'steve', created_at: 3 },
        { username: 'ALEX', created_at: 4 },
      ],
      expectedWhitelistCalls: [],
    },
    {
      name: 'addition only',
      id: 11,
      loaded: true,
      initial: [{ username: 'Alex', created_at: 1 }],
      current: [
        { username: 'Alex', created_at: 1 },
        { username: 'NewPlayer', created_at: 2 },
      ],
      expectedWhitelistCalls: [
        { method: 'addWhitelist', payload: { username: 'NewPlayer', endpoint_id: 11 } },
      ],
    },
    {
      name: 'removal only',
      id: 11,
      loaded: true,
      initial: [
        { username: 'Alex', created_at: 1 },
        { username: 'OldPlayer', created_at: 2 },
      ],
      current: [{ username: 'Alex', created_at: 1 }],
      expectedWhitelistCalls: [
        { method: 'removeWhitelist', username: 'OldPlayer', endpointId: 11 },
      ],
    },
    {
      name: 'additions and removals together',
      id: 11,
      loaded: true,
      initial: [
        { username: 'Keep', created_at: 1 },
        { username: 'RemoveOne', created_at: 2 },
        { username: 'RemoveTwo', created_at: 3 },
      ],
      current: [
        { username: 'keep', created_at: 4 },
        { username: 'AddOne', created_at: 5 },
        { username: 'AddTwo', created_at: 6 },
      ],
      expectedWhitelistCalls: [
        { method: 'addWhitelist', payload: { username: 'AddOne', endpoint_id: 11 } },
        { method: 'addWhitelist', payload: { username: 'AddTwo', endpoint_id: 11 } },
        { method: 'removeWhitelist', username: 'RemoveOne', endpointId: 11 },
        { method: 'removeWhitelist', username: 'RemoveTwo', endpointId: 11 },
      ],
    },
    {
      name: 'all entries removed',
      id: 11,
      loaded: true,
      initial: [
        { username: 'RemoveOne', created_at: 1 },
        { username: 'RemoveTwo', created_at: 2 },
      ],
      current: [],
      expectedWhitelistCalls: [
        { method: 'removeWhitelist', username: 'RemoveOne', endpointId: 11 },
        { method: 'removeWhitelist', username: 'RemoveTwo', endpointId: 11 },
      ],
    },
    {
      name: 'unloaded entries',
      id: 11,
      loaded: false,
      initial: [{ username: 'ServerState', created_at: 1 }],
      current: [{ username: 'UnsavedLocalState', created_at: 2 }],
      expectedWhitelistCalls: [],
    },
    {
      name: 'new endpoint entries',
      id: null,
      loaded: true,
      initial: [],
      current: [
        { username: 'FirstPlayer', created_at: 1 },
        { username: 'SecondPlayer', created_at: 2 },
      ],
      expectedWhitelistCalls: [
        { method: 'addWhitelist', payload: { username: 'FirstPlayer', endpoint_id: 51 } },
        { method: 'addWhitelist', payload: { username: 'SecondPlayer', endpoint_id: 51 } },
      ],
    },
  ])('sends exact whitelist packets for $name', async (testCase) => {
    const row = createRow({
      id: testCase.id,
      rowKey: testCase.id === null ? 'new-1' : `row-${testCase.id}`,
      _loaded: testCase.loaded,
      _initialWhitelist: testCase.initial,
      _whitelist: testCase.current,
    })
    const savedID = testCase.id ?? 51
    const savedSettings = {
      fallback_strategy: 'serial',
      fallback_probe_interval: 600,
      fallbacks: [endpointFromRow(row, savedID)],
    }
    const { calls, gateway } = createGateway(savedSettings)

    await saveFallbackConfiguration(
      { fallback_strategy: 'serial', fallback_probe_interval: 600 },
      [row],
      gateway,
    )

    expect(calls).toEqual([
      {
        method: 'saveSettings',
        payload: {
          fallback_strategy: 'serial',
          fallback_probe_interval: 600,
          fallbacks: [
            {
              id: testCase.id,
              priority: 1,
              session_url: 'https://one.example/session',
              account_url: 'https://one.example/account',
              services_url: 'https://one.example/services',
              cache_ttl: 60,
              enable_profile: true,
              enable_hasjoined: true,
              enable_whitelist: true,
              note: 'one',
              skin_domains: ['one.example', 'cdn.one.example'],
            },
          ],
        },
      },
      { method: 'loadSettings' },
      ...testCase.expectedWhitelistCalls,
    ])
    expect(row.id).toBe(savedID)
    expect(row._whitelist).toEqual(testCase.current)
    expect(row._initialWhitelist).toEqual(testCase.loaded ? testCase.current : testCase.initial)
    if (testCase.loaded) {
      expect(row._initialWhitelist).not.toBe(row._whitelist)
    }
  })

  it('rejects malformed or incomplete saved settings before sending whitelist packets', async () => {
    const row = createRow({
      _initialWhitelist: [],
      _whitelist: [{ username: 'Pending', created_at: 1 }],
    })
    const malformed = createGateway({ fallbacks: 'invalid' })
    await expect(
      saveFallbackConfiguration(
        { fallback_strategy: 'serial', fallback_probe_interval: 600 },
        [row],
        malformed.gateway,
      ),
    ).rejects.toThrow('invalid fallback settings response')
    expect(malformed.calls.map((call) => call.method)).toEqual(['saveSettings', 'loadSettings'])
    expect(row._initialWhitelist).toEqual([])

    const missing = createGateway({ fallbacks: [] })
    await expect(
      saveFallbackConfiguration(
        { fallback_strategy: 'serial', fallback_probe_interval: 600 },
        [row],
        missing.gateway,
      ),
    ).rejects.toThrow('saved fallback endpoint not found: one')
    expect(missing.calls.map((call) => call.method)).toEqual(['saveSettings', 'loadSettings'])
    expect(row._initialWhitelist).toEqual([])
  })

  it('does not advance the whitelist snapshot when a difference request fails', async () => {
    const row = createRow({
      _initialWhitelist: [{ username: 'Old', created_at: 1 }],
      _whitelist: [{ username: 'New', created_at: 2 }],
    })
    const calls: GatewayCall[] = []
    const gateway: FallbackSaveGateway = {
      async saveSettings(payload) {
        calls.push({ method: 'saveSettings', payload })
      },
      async loadSettings() {
        calls.push({ method: 'loadSettings' })
        return { fallbacks: [endpointFromRow(row, 11)] }
      },
      async addWhitelist(payload) {
        calls.push({ method: 'addWhitelist', payload })
        throw new Error('add failed')
      },
      async removeWhitelist(username, endpointId) {
        calls.push({ method: 'removeWhitelist', username, endpointId })
      },
    }

    await expect(
      saveFallbackConfiguration(
        { fallback_strategy: 'serial', fallback_probe_interval: 600 },
        [row],
        gateway,
      ),
    ).rejects.toThrow('add failed')
    expect(calls.slice(2)).toEqual([
      { method: 'addWhitelist', payload: { username: 'New', endpoint_id: 11 } },
      { method: 'removeWhitelist', username: 'Old', endpointId: 11 },
    ])
    expect(row._initialWhitelist).toEqual([{ username: 'Old', created_at: 1 }])
    expect(row._whitelist).toEqual([{ username: 'New', created_at: 2 }])
  })
})
