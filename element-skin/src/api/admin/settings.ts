import client from '../client'

export function getAdminSettingsGroup(group: string): Promise<{ data: Record<string, unknown> }> {
  return client.get(`/v1/admin/settings/${group}`)
}

export function saveAdminSettingsGroup(group: string, data: Record<string, unknown>): Promise<{ data: { ok: boolean } }> {
  return client.post(`/v1/admin/settings/${group}`, data)
}
