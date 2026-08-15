import client from './client'
import type { ItemListResponse, OfficialProfileBinding } from './types'

export function getOfficialProfileBindings(): Promise<{
  data: ItemListResponse<OfficialProfileBinding>
}> {
  return client.get('/v2/users/me/official-profile-bindings')
}

export function createOfficialProfileBinding(data: {
  identity_id: string
}): Promise<{ data: OfficialProfileBinding }> {
  return client.post('/v2/users/me/official-profile-bindings', data)
}

export function syncOfficialProfileBinding(
  bindingId: string,
): Promise<{ data: OfficialProfileBinding }> {
  return client.post(`/v2/users/me/official-profile-bindings/${bindingId}/sync`)
}

export function deleteOfficialProfileBinding(bindingId: string): Promise<{ data: void }> {
  return client.delete(`/v2/users/me/official-profile-bindings/${bindingId}`)
}
