import { getAdminSettingsGroup, saveAdminSettingsGroup } from '@/api/admin/settings'
import { addWhitelistUser, removeWhitelistUser } from '@/api/admin/whitelist'
import type { FallbackEndpoint, FallbackRow } from '@/components/admin/mojang/types'
import type { FallbackSettingsForm } from '@/components/admin/mojang/fallbackSettings'
import {
  findSavedEndpoint,
  toFallbackSettingsPayload,
} from '@/components/admin/mojang/fallbackSettings'
import { getWhitelistChanges } from '@/components/admin/mojang/whitelist'

export interface FallbackSaveGateway {
  saveSettings(payload: Record<string, unknown>): Promise<void>
  loadSettings(): Promise<Record<string, unknown>>
  addWhitelist(input: { username: string; endpoint_id: number }): Promise<void>
  removeWhitelist(username: string, endpointId: number): Promise<void>
}

const httpGateway: FallbackSaveGateway = {
  async saveSettings(payload) {
    await saveAdminSettingsGroup('fallback', payload)
  },
  async loadSettings() {
    const response = await getAdminSettingsGroup('fallback')
    return response.data
  },
  async addWhitelist(input) {
    await addWhitelistUser(input)
  },
  async removeWhitelist(username, endpointId) {
    await removeWhitelistUser(username, endpointId)
  },
}

export async function saveFallbackConfiguration(
  settings: FallbackSettingsForm,
  rows: FallbackRow[],
  gateway: FallbackSaveGateway = httpGateway,
): Promise<Record<string, unknown>> {
  await gateway.saveSettings(toFallbackSettingsPayload(settings, rows))
  const savedSettings = await gateway.loadSettings()
  if (!Array.isArray(savedSettings.fallbacks)) {
    throw new Error('invalid fallback settings response')
  }

  const savedEndpoints = savedSettings.fallbacks as FallbackEndpoint[]
  const resolvedRows = rows.map((row) => {
    const savedEndpoint = findSavedEndpoint(row, savedEndpoints)
    if (!savedEndpoint || !savedEndpoint.id) {
      throw new Error(`saved fallback endpoint not found: ${row.note || row.session_url}`)
    }
    return { row, savedEndpoint }
  })

  for (const { row, savedEndpoint } of resolvedRows) {
    row.id = savedEndpoint.id
    if (!row._loaded) continue

    const { toAdd, toRemove } = getWhitelistChanges(row)
    await Promise.all([
      ...toAdd.map((entry) =>
        gateway.addWhitelist({ username: entry.username, endpoint_id: savedEndpoint.id! }),
      ),
      ...toRemove.map((entry) => gateway.removeWhitelist(entry.username, savedEndpoint.id!)),
    ])
    row._initialWhitelist = row._whitelist.map((entry) => ({ ...entry }))
  }
  return savedSettings
}
