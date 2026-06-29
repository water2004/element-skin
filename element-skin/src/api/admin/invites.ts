import client from '../client'
import type { Invite } from '../types'

export function getAdminInvites(params: { cursor?: string | null; limit?: number }): Promise<{
  data: { items: Invite[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v1/admin/invites', { params })
}

export function createAdminInvite(data: { code?: string; total_uses?: number | null; note?: string }): Promise<{
  data: { code: string; total_uses: number; note: string }
}> {
  return client.post('/v1/admin/invites', data)
}

export function deleteAdminInvite(code: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/admin/invites/${code}`)
}
