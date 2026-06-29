import client from '../client'
import type { WhitelistEntry } from '../types'

export function getWhitelist(endpointId: number): Promise<{ data: WhitelistEntry[] }> {
  return client.get('/v1/admin/official-whitelist', { params: { endpoint_id: endpointId } })
}

export function addWhitelistUser(data: { username: string; endpoint_id: number }): Promise<{ data: { ok: boolean } }> {
  return client.post('/v1/admin/official-whitelist', data)
}

export function removeWhitelistUser(username: string, endpointId: number): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/admin/official-whitelist/${username}`, { params: { endpoint_id: endpointId } })
}
