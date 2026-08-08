import {
  approveOAuthAuthorization,
  clearOAuthClientPermission,
  createOAuthApp,
  decideDeviceAuthorization,
  deleteOAuthApp,
  getAdminOAuthApp,
  getDeviceAuthorization,
  getOAuthAuthorizationDetails,
  getOAuthApp,
  getOAuthClientPermissions,
  getOAuthWebhookEventCatalog,
  getPermissionCatalog,
  listAdminOAuthApps,
  listOAuthApps,
  listOAuthGrants,
  revokeOAuthGrant,
  rotateOAuthSecret,
  reviewAdminOAuthApp,
  setOAuthClientPermission,
  submitOAuthAppReview,
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
  webhook_endpoints: [],
}

const authorizationRequest = {
  response_type: 'code',
  client_id: 'client-1',
  redirect_uri: 'https://app.example/callback',
  scope: 'account.read.self profile.read.owned',
  state: 'opaque-state',
  code_challenge: 'challenge',
  code_challenge_method: 'S256',
}

export function oauthApiCases(): ApiCase[] {
  return [
    {
      name: 'listOAuthApps gets app list with limit',
      method: 'get',
      call: () => listOAuthApps(25),
      args: ['/v2/oauth/apps', { params: { limit: 25 } }],
    },
    {
      name: 'createOAuthApp posts app payload',
      method: 'post',
      call: () => createOAuthApp(oauthPayload),
      args: ['/v2/oauth/apps', oauthPayload],
    },
    {
      name: 'getOAuthApp gets owned app detail',
      method: 'get',
      call: () => getOAuthApp('client-1'),
      args: ['/v2/oauth/apps/client-1'],
    },
    {
      name: 'getOAuthWebhookEventCatalog gets subscribable event definitions',
      method: 'get',
      call: getOAuthWebhookEventCatalog,
      args: ['/v2/oauth/webhook-events'],
    },
    {
      name: 'listOAuthGrants gets grant list with limit',
      method: 'get',
      call: () => listOAuthGrants(15),
      args: ['/v2/oauth/grants', { params: { limit: 15 } }],
    },
    {
      name: 'revokeOAuthGrant deletes one grant',
      method: 'delete',
      call: () => revokeOAuthGrant('grant-1'),
      args: ['/v2/oauth/grants/grant-1'],
    },
    {
      name: 'updateOAuthApp patches app payload',
      method: 'patch',
      call: () => updateOAuthApp('client-1', { ...oauthPayload, status: 'disabled' }),
      args: ['/v2/oauth/apps/client-1', { ...oauthPayload, status: 'disabled' }],
    },
    {
      name: 'deleteOAuthApp deletes app',
      method: 'delete',
      call: () => deleteOAuthApp('client-1'),
      args: ['/v2/oauth/apps/client-1'],
    },
    {
      name: 'rotateOAuthSecret posts secret rotation',
      method: 'post',
      call: () => rotateOAuthSecret('client-1'),
      args: ['/v2/oauth/apps/client-1/secret'],
    },
    {
      name: 'submitOAuthAppReview posts review submission',
      method: 'post',
      call: () => submitOAuthAppReview('client-1'),
      args: ['/v2/oauth/apps/client-1/review-submission'],
    },
    {
      name: 'getPermissionCatalog gets catalog',
      method: 'get',
      call: getPermissionCatalog,
      args: ['/v2/permissions/catalog'],
    },
    {
      name: 'getOAuthClientPermissions gets client subject permissions',
      method: 'get',
      call: () => getOAuthClientPermissions('client-1'),
      args: ['/v2/oauth/apps/client-1/permissions'],
    },
    {
      name: 'setOAuthClientPermission puts permission override',
      method: 'put',
      call: () =>
        setOAuthClientPermission('client-1', 'minecraft_session.hasjoined.server', 'allow'),
      args: [
        '/v2/oauth/apps/client-1/permissions/minecraft_session.hasjoined.server',
        { effect: 'allow' },
      ],
    },
    {
      name: 'clearOAuthClientPermission deletes permission override',
      method: 'delete',
      call: () => clearOAuthClientPermission('client-1', 'minecraft_session.hasjoined.server'),
      args: ['/v2/oauth/apps/client-1/permissions/minecraft_session.hasjoined.server'],
    },
    {
      name: 'listAdminOAuthApps gets filtered lightweight admin list',
      method: 'get',
      call: () => listAdminOAuthApps('pending', 20),
      args: ['/v2/admin/oauth/apps', { params: { status: 'pending', limit: 20 } }],
    },
    {
      name: 'getAdminOAuthApp gets admin app detail on demand',
      method: 'get',
      call: () => getAdminOAuthApp('client-1'),
      args: ['/v2/admin/oauth/apps/client-1'],
    },
    {
      name: 'reviewAdminOAuthApp patches admin review status',
      method: 'patch',
      call: () => reviewAdminOAuthApp('client-1', 'rejected', 'Missing support contact'),
      args: [
        '/v2/admin/oauth/apps/client-1/review',
        { status: 'rejected', reason: 'Missing support contact' },
      ],
    },
    {
      name: 'getOAuthAuthorizationDetails gets authorization request details',
      method: 'get',
      call: () => getOAuthAuthorizationDetails(authorizationRequest),
      args: ['/oauth/authorize', { params: authorizationRequest }],
    },
    {
      name: 'approveOAuthAuthorization posts authorization approval',
      method: 'post',
      call: () => approveOAuthAuthorization(authorizationRequest),
      args: ['/oauth/authorize', authorizationRequest],
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
