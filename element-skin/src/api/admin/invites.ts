import client from '../client'
import type { Invite } from '../types'
import { encodeBase64URL } from '@/utils/base64url'

export function getAdminInvites(params: { cursor?: string | null; limit?: number }): Promise<{
  data: { items: Invite[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v2/admin/invites', { params })
}

export function createAdminInvite(data: {
  code?: string
  total_uses?: number | null
  note?: string
}): Promise<{
  data: { code: string; total_uses: number | null; note: string }
}> {
  const { code, ...fields } = data
  return client.post('/v2/admin/invites', {
    ...fields,
    ...(code === undefined ? {} : { code_base64: encodeBase64URL(code) }),
  })
}

export function deleteAdminInvite(code: string): Promise<{ data: void }> {
  return client.delete(`/v2/admin/invites/${encodeBase64URL(code)}`)
}
