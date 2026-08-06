import client from '../client'

import type { EmailSuffixPolicy } from '../types'

export function getAdminSettingsGroup(group: string): Promise<{ data: Record<string, unknown> }> {
  return client.get(`/v2/admin/settings/${group}`)
}

export function getAdminEmailSuffixPolicy(): Promise<{ data: EmailSuffixPolicy }> {
  return client.get('/v2/admin/settings/email-suffix-policy')
}

export function putAdminEmailSuffixPolicy(policy: EmailSuffixPolicy): Promise<{ data: void }> {
  return client.put('/v2/admin/settings/email-suffix-policy', policy)
}

export function saveAdminSettingsGroup(
  group: string,
  data: Record<string, unknown>,
): Promise<{ data: void }> {
  return client.post(`/v2/admin/settings/${group}`, data)
}
