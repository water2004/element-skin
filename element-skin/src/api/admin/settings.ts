import client from '../client'

// Legacy: get all settings at once
export function getAdminSettings(): Promise<{ data: Record<string, unknown> }> {
  return client.get('/admin/settings')
}

// Legacy: save all settings at once
export function saveAdminSettings(data: Record<string, unknown>): Promise<{ data: { ok: boolean } }> {
  return client.post('/admin/settings', data)
}

// Granular: get settings group
export function getAdminSettingsGroup(group: string): Promise<{ data: Record<string, unknown> }> {
  return client.get(`/admin/settings/${group}`)
}

// Granular: save settings group
export function saveAdminSettingsGroup(group: string, data: Record<string, unknown>): Promise<{ data: { ok: boolean } }> {
  return client.post(`/admin/settings/${group}`, data)
}
