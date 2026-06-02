import client from './client'
import type { MicrosoftAuthUrlResponse } from './types'

export function getMicrosoftAuthUrl(): Promise<{ data: MicrosoftAuthUrlResponse }> {
  return client.get('/microsoft/auth-url')
}

export function getMicrosoftProfile(data: { ms_token: string }): Promise<{ data: any }> {
  return client.post('/microsoft/get-profile', data)
}

// 导入只凭 get-profile 换发的一次性 import_token，资料由服务端固化，前端不再传 profile 字段。
export function importMicrosoftProfile(data: {
  ms_token: string
}): Promise<{ data: { ok: boolean } }> {
  return client.post('/microsoft/import-profile', data)
}
