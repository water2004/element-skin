import client from './client'
import type { Profile } from './types'

export function getProfiles(params: { cursor?: string | null; limit?: number }): Promise<{
  data: { items: Profile[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v1/users/me/profiles', { params })
}

export function createProfile(data: { name: string; model?: string }): Promise<{ data: { id: string; name: string; model: string } }> {
  return client.post('/v1/users/me/profiles', data)
}

export function patchProfile(pid: string, data: { name?: string }): Promise<{ data: { ok: boolean } }> {
  return client.patch(`/v1/users/me/profiles/${pid}`, data)
}

export function deleteProfile(pid: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/users/me/profiles/${pid}`)
}

export function clearProfileSkin(pid: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/users/me/profiles/${pid}/skin`)
}

export function clearProfileCape(pid: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/users/me/profiles/${pid}/cape`)
}
