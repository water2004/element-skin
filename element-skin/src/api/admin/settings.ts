import client from '../client'

export function getAdminSettingsGroup(group: string): Promise<{ data: Record<string, unknown> }> {
  return client.get(`/admin/settings/${group}`)
}

export function saveAdminSettingsGroup(group: string, data: Record<string, unknown>): Promise<{ data: { ok: boolean } }> {
  return client.post(`/admin/settings/${group}`, data)
}
