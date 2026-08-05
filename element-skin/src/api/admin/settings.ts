import client from '../client'

export function getAdminSettingsGroup(group: string): Promise<{ data: Record<string, unknown> }> {
  return client.get(`/v2/admin/settings/${group}`)
}

export function saveAdminSettingsGroup(
  group: string,
  data: Record<string, unknown>,
): Promise<{ data: void }> {
  return client.post(`/v2/admin/settings/${group}`, data)
}
