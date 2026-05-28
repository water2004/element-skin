import client from '../client'
import type { WhitelistEntry } from '../types'

export function getWhitelist(endpointId: number): Promise<{ data: WhitelistEntry[] }> {
  return client.get('/admin/official-whitelist', { params: { endpoint_id: endpointId } })
}

export function addWhitelistUser(data: { username: string; endpoint_id: number }): Promise<{ data: { ok: boolean } }> {
  return client.post('/admin/official-whitelist', data)
}

export function removeWhitelistUser(username: string, endpointId: number): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/admin/official-whitelist/${username}`, { params: { endpoint_id: endpointId } })
}
