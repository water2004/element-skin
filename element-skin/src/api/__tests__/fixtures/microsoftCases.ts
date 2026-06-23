import { getMicrosoftAuthUrl, getMicrosoftProfile, importMicrosoftProfile } from '../../microsoft'
import type { ApiCase } from './types'

export function microsoftApiCases(): ApiCase[] {
  return [
    {
      name: 'getMicrosoftAuthUrl gets auth URL',
      method: 'get',
      call: getMicrosoftAuthUrl,
      args: ['/microsoft/auth-url'],
    },
    {
      name: 'getMicrosoftProfile posts ms token',
      method: 'post',
      call: () => getMicrosoftProfile({ ms_token: 'profile-token' }),
      args: ['/microsoft/get-profile', { ms_token: 'profile-token' }],
    },
    {
      name: 'importMicrosoftProfile posts import token only',
      method: 'post',
      call: () => importMicrosoftProfile({ ms_token: 'import-token' }),
      args: ['/microsoft/import-profile', { ms_token: 'import-token' }],
    },
  ]
}
