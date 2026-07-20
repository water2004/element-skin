import { describe, expect, it } from 'vitest'

import type { FallbackRow } from '@/components/admin/mojang/types'
import {
  createWhitelistEntryDraft,
  getWhitelistChanges,
  hasWhitelistChanges,
  setLoadedWhitelist,
} from '@/components/admin/mojang/whitelist'

const baseRow: FallbackRow = {
  id: 1,
  rowKey: 1,
  priority: 1,
  session_url: 'https://session.example',
  account_url: 'https://account.example',
  services_url: 'https://services.example',
  cache_ttl: 600,
  enable_profile: true,
  enable_hasjoined: true,
  enable_whitelist: true,
  note: 'Primary',
  skin_domains_text: 'textures.example',
  _whitelist: [],
  _initialWhitelist: [],
  _new_user: '',
  _loaded: true,
}

function rowWithLists(initial: string[], current: string[], loaded = true): FallbackRow {
  return {
    ...baseRow,
    _initialWhitelist: initial.map((username, index) => ({
      username,
      created_at: 1000 + index,
    })),
    _whitelist: current.map((username, index) => ({
      username,
      created_at: 2000 + index,
    })),
    _loaded: loaded,
  }
}

describe('whitelist state', () => {
  it('loads response items into independent current and initial lists', () => {
    const row = rowWithLists([], [], false)
    const entries = [
      { username: 'Steve', created_at: 1000 },
      { username: 'Alex', created_at: 2000 },
    ]

    setLoadedWhitelist(row, entries)

    expect(row._loaded).toBe(true)
    expect(row._whitelist).toEqual(entries)
    expect(row._initialWhitelist).toEqual(entries)
    expect(row._whitelist).not.toBe(entries)
    expect(row._initialWhitelist).not.toBe(entries)
    expect(row._initialWhitelist).not.toBe(row._whitelist)

    row._whitelist[0]!.username = 'Changed'
    expect(entries[0]!.username).toBe('Steve')
    expect(row._initialWhitelist[0]!.username).toBe('Steve')
  })

  it('loads an empty response as exact empty whitelist state', () => {
    const row = rowWithLists(['Steve'], ['Alex'], false)

    setLoadedWhitelist(row, [])

    expect(row).toMatchObject({
      _loaded: true,
      _whitelist: [],
      _initialWhitelist: [],
    })
    expect(hasWhitelistChanges(row)).toBe(false)
  })

  it('does not report changes before whitelist is loaded', () => {
    const row = rowWithLists(['Steve'], ['Alex'], false)

    expect(hasWhitelistChanges(row)).toBe(false)
  })

  it('compares names case-insensitively and ignores ordering', () => {
    const row = rowWithLists(['Steve', 'Alex'], ['alex', 'STEVE'])

    expect(hasWhitelistChanges(row)).toBe(false)
  })

  it('reports added and removed users exactly', () => {
    const added = rowWithLists(['Steve'], ['Steve', 'Alex'])
    const removed = rowWithLists(['Steve', 'Alex'], ['Steve'])

    expect(hasWhitelistChanges(added)).toBe(true)
    expect(hasWhitelistChanges(removed)).toBe(true)
  })

  it('returns exact added and removed entries', () => {
    const row = rowWithLists(['Steve', 'Alex'], ['alex', 'Herobrine'])

    expect(getWhitelistChanges(row)).toEqual({
      toAdd: [{ username: 'Herobrine', created_at: 2001 }],
      toRemove: [{ username: 'Steve', created_at: 1000 }],
    })
  })

  it('creates trimmed whitelist entry drafts with exact timestamps', () => {
    const row = rowWithLists([], ['Alex'])

    expect(createWhitelistEntryDraft(row, ' Steve ', 123456)).toEqual({
      ok: true,
      entry: {
        username: 'Steve',
        created_at: 123456,
      },
    })
  })

  it('rejects empty and duplicate whitelist entry drafts exactly', () => {
    const row = rowWithLists([], ['Alex'])

    expect(createWhitelistEntryDraft(row, '   ', 123456)).toEqual({
      ok: false,
      reason: 'empty',
    })
    expect(createWhitelistEntryDraft(row, 'alex', 123456)).toEqual({
      ok: false,
      reason: 'duplicate',
    })
  })
})
