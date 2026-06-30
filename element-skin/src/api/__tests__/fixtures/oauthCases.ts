import {
  clearOAuthClientPermission,
  createOAuthApp,
  decideDeviceAuthorization,
  deleteOAuthApp,
  getDeviceAuthorization,
  getOAuthClientPermissions,
  getPermissionCatalog,
  listOAuthApps,
  rotateOAuthSecret,
  setOAuthClientPermission,
  updateOAuthApp,
} from '../../oauth'
import type { ApiCase } from './types'

const oauthPayload = {
  name: 'Launcher',
  description: 'Launcher integration',
  redirect_uri: 'https://app.example/callback',
  website_url: 'https://app.example',
  client_type: 'confidential' as const,
  permissions: ['account.read.self'],
}

export function oauthApiCases(): ApiCase[] {
  return [
    {
      name: 'listOAuthApps gets app list with limit',
      method: 'get',
      call: () => listOAuthApps(25),
      args: ['/v1/oauth/apps', { params: { limit: 25 } }],
    },
    {
      name: 'createOAuthApp posts app payload',
      method: 'post',
      call: () => createOAuthApp(oauthPayload),
      args: ['/v1/oauth/apps', oauthPayload],
    },
    {
      name: 'updateOAuthApp patches app payload',
      method: 'patch',
      call: () => updateOAuthApp('client-1', { ...oauthPayload, status: 'disabled' }),
      args: ['/v1/oauth/apps/client-1', { ...oauthPayload, status: 'disabled' }],
    },
    {
      name: 'deleteOAuthApp deletes app',
      method: 'delete',
      call: () => deleteOAuthApp('client-1'),
      args: ['/v1/oauth/apps/client-1'],
    },
    {
      name: 'rotateOAuthSecret posts secret rotation',
      method: 'post',
      call: () => rotateOAuthSecret('client-1'),
      args: ['/v1/oauth/apps/client-1/secret'],
    },
    {
      name: 'getPermissionCatalog gets catalog',
      method: 'get',
      call: getPermissionCatalog,
      args: ['/v1/permissions/catalog'],
    },
    {
      name: 'getOAuthClientPermissions gets client subject permissions',
      method: 'get',
      call: () => getOAuthClientPermissions('client-1'),
      args: ['/v1/oauth/apps/client-1/permissions'],
    },
    {
      name: 'setOAuthClientPermission puts permission override',
      method: 'put',
      call: () => setOAuthClientPermission('client-1', 'minecraft_session.hasjoined.server', 'allow'),
      args: [
        '/v1/oauth/apps/client-1/permissions/minecraft_session.hasjoined.server',
        { effect: 'allow' },
      ],
    },
    {
      name: 'clearOAuthClientPermission deletes permission override',
      method: 'delete',
      call: () => clearOAuthClientPermission('client-1', 'minecraft_session.hasjoined.server'),
      args: ['/v1/oauth/apps/client-1/permissions/minecraft_session.hasjoined.server'],
    },
    {
      name: 'getDeviceAuthorization gets user code details',
      method: 'get',
      call: () => getDeviceAuthorization('ABCD-1234'),
      args: ['/oauth/device', { params: { user_code: 'ABCD-1234' } }],
    },
    {
      name: 'decideDeviceAuthorization posts decision',
      method: 'post',
      call: () => decideDeviceAuthorization('ABCD-1234', true),
      args: ['/oauth/device', { user_code: 'ABCD-1234', approve: true }],
    },
  ]
}
