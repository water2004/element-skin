import client from '../client'
import type { Texture } from '../types'

export function getAdminTextures(params: {
  cursor?: string | null
  limit?: number
  q?: string
  type?: string
}): Promise<{
  data: { items: Texture[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v1/admin/textures', { params })
}

export function patchAdminTexture(hash: string, data: { type: string; model?: string; note?: string; is_public?: boolean | number }): Promise<{ data: { ok: boolean } }> {
  return client.patch(`/v1/admin/textures/${hash}`, data)
}

export function deleteAdminTexture(hash: string, params: { type?: string; user_id?: string; force?: boolean }): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/admin/textures/${hash}`, { params })
}
