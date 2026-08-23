import client from '../client'
import type { Profile } from '../types'

export function getAdminProfiles(params: { cursor?: string | null; limit?: number; q?: string }): Promise<{
  data: { items: Profile[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v2/admin/profiles', { params })
}

export function patchAdminProfile(profileId: string, data: { name?: string }): Promise<{ data: void }> {
  return client.patch(`/v2/admin/profiles/${profileId}`, data)
}

export function deleteAdminProfile(profileId: string): Promise<{ data: void }> {
  return client.delete(`/v2/admin/profiles/${profileId}`)
}

export function patchProfileSkin(profileId: string, data: { hash?: string | null }): Promise<{ data: void }> {
  return client.patch(`/v2/admin/profiles/${profileId}/skin`, data)
}

export function patchProfileCape(profileId: string, data: { hash?: string | null }): Promise<{ data: void }> {
  return client.patch(`/v2/admin/profiles/${profileId}/cape`, data)
}
