import client from './client'
import type { YggdrasilImportResult } from './types'

export function getRemoteYggProfiles(data: {
  api_url: string
  username: string
  password: string
}): Promise<{ data: { profiles: Array<{ id: string; name: string }> } }> {
  return client.post('/v1/imports/remote-ygg/profiles/preview', data)
}

export function importRemoteYggProfiles(data: {
  api_url: string
  profiles: Array<{ profile_id: string; profile_name: string }>
}): Promise<{ data: YggdrasilImportResult }> {
  return client.post('/v1/imports/remote-ygg/profiles/import-batch', data)
}
