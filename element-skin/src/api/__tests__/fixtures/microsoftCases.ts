import { getMicrosoftAuthUrl, getMicrosoftProfile, importMicrosoftProfile } from '../../microsoft'
import type { ApiCase } from './types'

export function microsoftApiCases(): ApiCase[] {
  return [
    {
      name: 'getMicrosoftAuthUrl gets auth URL',
      method: 'get',
      call: getMicrosoftAuthUrl,
      args: ['/v1/imports/microsoft/auth-url'],
    },
    {
      name: 'getMicrosoftProfile posts ms token',
      method: 'post',
      call: () => getMicrosoftProfile({ ms_token: 'profile-token' }),
      args: ['/v1/imports/microsoft/profile', { ms_token: 'profile-token' }],
    },
    {
      name: 'importMicrosoftProfile posts import token only',
      method: 'post',
      call: () => importMicrosoftProfile({ ms_token: 'import-token' }),
      args: ['/v1/imports/microsoft/profile/import', { ms_token: 'import-token' }],
    },
  ]
}
