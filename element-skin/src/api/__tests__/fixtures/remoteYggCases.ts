import { getRemoteYggProfiles, importRemoteYggProfiles } from '../../remote-ygg'
import type { ApiCase } from './types'

export function remoteYggApiCases(): ApiCase[] {
  return [
    {
      name: 'getRemoteYggProfiles posts remote credentials',
      method: 'post',
      call: () =>
        getRemoteYggProfiles({
          api_url: 'https://ygg.example/api',
          username: 'remote-user',
          password: 'remote-password',
        }),
      args: [
        '/v1/imports/remote-ygg/profiles/preview',
        {
          api_url: 'https://ygg.example/api',
          username: 'remote-user',
          password: 'remote-password',
        },
      ],
    },
    {
      name: 'importRemoteYggProfiles posts selected profiles',
      method: 'post',
      call: () =>
        importRemoteYggProfiles({
          api_url: 'https://ygg.example/api',
          profiles: [{ profile_id: 'p1', profile_name: 'RemotePlayer' }],
        }),
      args: [
        '/v1/imports/remote-ygg/profiles/import-batch',
        {
          api_url: 'https://ygg.example/api',
          profiles: [{ profile_id: 'p1', profile_name: 'RemotePlayer' }],
        },
      ],
    },
  ]
}
