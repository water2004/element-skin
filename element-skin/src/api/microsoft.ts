import client from './client'
import type { MicrosoftAuthUrlResponse, MicrosoftProfileResponse } from './types'

export function getMicrosoftAuthUrl(): Promise<{ data: MicrosoftAuthUrlResponse }> {
  return client.get('/v1/imports/microsoft/auth-url')
}

export function getMicrosoftProfile(data: { ms_token: string }): Promise<{ data: MicrosoftProfileResponse }> {
  return client.post('/v1/imports/microsoft/profile', data)
}

// 导入只凭 get-profile 换发的一次性 import_token，资料由服务端固化，前端不再传 profile 字段。
export function importMicrosoftProfile(data: {
  ms_token: string
}): Promise<{ data: { ok: boolean } }> {
  return client.post('/v1/imports/microsoft/profile/import', data)
}
