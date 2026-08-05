import client from './client'
import type { Profile } from './types'

export function getProfiles(params: { cursor?: string | null; limit?: number }): Promise<{
  data: { items: Profile[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v2/users/me/profiles', { params })
}

export function createProfile(data: { name: string; model?: string }): Promise<{ data: { id: string; name: string; model: string } }> {
  return client.post('/v2/users/me/profiles', data)
}

export function patchProfile(pid: string, data: { name?: string }): Promise<{ data: void }> {
  return client.patch(`/v2/users/me/profiles/${pid}`, data)
}

export function deleteProfile(pid: string): Promise<{ data: void }> {
  return client.delete(`/v2/users/me/profiles/${pid}`)
}

export function clearProfileSkin(pid: string): Promise<{ data: void }> {
  return client.delete(`/v2/users/me/profiles/${pid}/skin`)
}

export function clearProfileCape(pid: string): Promise<{ data: void }> {
  return client.delete(`/v2/users/me/profiles/${pid}/cape`)
}
