import client from '../client'
import type { Invite } from '../types'

export function getAdminInvites(params: { cursor?: string | null; limit?: number }): Promise<{
  data: { items: Invite[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v2/admin/invites', { params })
}

export function createAdminInvite(data: { code?: string; total_uses?: number | null; note?: string }): Promise<{
  data: { code: string; total_uses: number | null; note: string }
}> {
  return client.post('/v2/admin/invites', data)
}

export function deleteAdminInvite(code: string): Promise<{ data: void }> {
  return client.delete(`/v2/admin/invites/${code}`)
}
