import client from '../client'
import type { WhitelistResponse } from '../types'

export function getWhitelist(endpointId: number): Promise<{ data: WhitelistResponse }> {
  return client.get('/v2/admin/official-whitelist', { params: { endpoint_id: endpointId } })
}

export function addWhitelistUser(data: {
  username: string
  endpoint_id: number
}): Promise<{ data: { username: string; endpoint_id: number } }> {
  return client.post('/v2/admin/official-whitelist', data)
}

export function removeWhitelistUser(
  username: string,
  endpointId: number,
): Promise<{ data: void }> {
  return client.delete(`/v2/admin/official-whitelist/${username}`, { params: { endpoint_id: endpointId } })
}
