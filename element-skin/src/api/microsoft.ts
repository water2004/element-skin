import client from './client'
import type { MicrosoftAuthUrlResponse } from './types'

export function getMicrosoftAuthUrl(): Promise<{ data: MicrosoftAuthUrlResponse }> {
  return client.get('/microsoft/auth-url')
}

export function getMicrosoftProfile(data: { ms_token: string }): Promise<{ data: any }> {
  return client.post('/microsoft/get-profile', data)
}

export function importMicrosoftProfile(data: {
  profile_id: string
  profile_name: string
  skin_url?: string | null
  skin_variant?: string
  cape_url?: string | null
}): Promise<{ data: { ok: boolean } }> {
  return client.post('/microsoft/import-profile', data)
}
