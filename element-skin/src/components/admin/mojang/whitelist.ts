import type { FallbackRow } from '@/components/admin/mojang/types'
import type { WhitelistEntry } from '@/api/types'

export type WhitelistEntryDraftResult =
  | { ok: true; entry: WhitelistEntry }
  | { ok: false; reason: 'empty' | 'duplicate' }

export function hasWhitelistChanges(row: FallbackRow) {
  const changes = getWhitelistChanges(row)
  return changes.toAdd.length > 0 || changes.toRemove.length > 0
}

export function setLoadedWhitelist(row: FallbackRow, entries: WhitelistEntry[]): void {
  row._whitelist = entries.map((entry) => ({ ...entry }))
  row._initialWhitelist = entries.map((entry) => ({ ...entry }))
  row._loaded = true
}

export function getWhitelistChanges(row: FallbackRow): {
  toAdd: WhitelistEntry[]
  toRemove: WhitelistEntry[]
} {
  if (!row._loaded) return { toAdd: [], toRemove: [] }

  const initialNames = row._initialWhitelist.map((entry) => entry.username.toLowerCase())
  const currentNames = row._whitelist.map((entry) => entry.username.toLowerCase())

  return {
    toAdd: row._whitelist.filter(
      (entry) => !initialNames.includes(entry.username.toLowerCase()),
    ),
    toRemove: row._initialWhitelist.filter(
      (entry) => !currentNames.includes(entry.username.toLowerCase()),
    ),
  }
}

export function createWhitelistEntryDraft(
  row: FallbackRow,
  rawUsername: string,
  now = Date.now(),
): WhitelistEntryDraftResult {
  const username = rawUsername.trim()
  if (!username) return { ok: false, reason: 'empty' }

  const exists = row._whitelist.some(
    (entry) => entry.username.toLowerCase() === username.toLowerCase(),
  )
  if (exists) return { ok: false, reason: 'duplicate' }

  return {
    ok: true,
    entry: {
      username,
      created_at: now,
    },
  }
}
