import {
  deleteExternalIdentity,
  getExternalIdentities,
  getIdentityProviders,
  patchExternalIdentity,
  startIdentityAuthorization,
} from '../../identity'
import {
  createOfficialProfileBinding,
  deleteOfficialProfileBinding,
  getOfficialProfileBindings,
  syncOfficialProfileBinding,
} from '../../official-profiles'
import {
  createIdentityProvider,
  deleteIdentityProvider,
  getAdminIdentityProvider,
  getAdminIdentityProviders,
  updateIdentityProvider,
  type IdentityProviderInput,
} from '../../admin/identity-providers'
import type { ApiCase } from './types'

const providerInput: IdentityProviderInput = {
  name: 'Microsoft',
  issuer_url: 'https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0',
  client_id: 'client-id',
  client_secret: 'client-secret',
  scopes: ['openid', 'profile', 'XboxLive.signin', 'offline_access'],
  adapter: 'microsoft',
  icon_url: 'https://example.com/microsoft.svg',
  enabled: true,
  login_enabled: true,
  link_enabled: true,
  display_order: 10,
}

export function identityApiCases(): ApiCase[] {
  return [
    {
      name: 'getIdentityProviders gets public OIDC providers',
      method: 'get',
      call: getIdentityProviders,
      args: ['/v2/auth/identity-providers'],
    },
    {
      name: 'startIdentityAuthorization posts provider and intent',
      method: 'post',
      call: () =>
        startIdentityAuthorization({
          provider_id: 'provider-1',
          intent: 'link',
        }),
      args: ['/v2/identity-authorizations', { provider_id: 'provider-1', intent: 'link' }],
    },
    {
      name: 'startIdentityAuthorization preserves an internal login return target',
      method: 'post',
      call: () =>
        startIdentityAuthorization({
          provider_id: 'provider-1',
          intent: 'login',
          return_to: '/oauth/authorize?client_id=client-1&state=opaque',
        }),
      args: [
        '/v2/identity-authorizations',
        {
          provider_id: 'provider-1',
          intent: 'login',
          return_to: '/oauth/authorize?client_id=client-1&state=opaque',
        },
      ],
    },
    {
      name: 'startIdentityAuthorization targets one identity when reconnecting',
      method: 'post',
      call: () =>
        startIdentityAuthorization({
          provider_id: 'provider-1',
          intent: 'link',
          identity_id: 'identity-1',
        }),
      args: [
        '/v2/identity-authorizations',
        {
          provider_id: 'provider-1',
          intent: 'link',
          identity_id: 'identity-1',
        },
      ],
    },
    {
      name: 'getExternalIdentities gets owned identities',
      method: 'get',
      call: getExternalIdentities,
      args: ['/v2/users/me/identities'],
    },
    {
      name: 'patchExternalIdentity patches identity label only',
      method: 'patch',
      call: () => patchExternalIdentity('identity-1', { label: 'Personal' }),
      args: ['/v2/users/me/identities/identity-1', { label: 'Personal' }],
    },
    {
      name: 'deleteExternalIdentity deletes identity only',
      method: 'delete',
      call: () => deleteExternalIdentity('identity-1'),
      args: ['/v2/users/me/identities/identity-1'],
    },
    {
      name: 'getOfficialProfileBindings gets separate binding resources',
      method: 'get',
      call: getOfficialProfileBindings,
      args: ['/v2/users/me/official-profile-bindings'],
    },
    {
      name: 'createOfficialProfileBinding posts the selected Microsoft identity',
      method: 'post',
      call: () => createOfficialProfileBinding({ identity_id: 'identity-1' }),
      args: ['/v2/users/me/official-profile-bindings', { identity_id: 'identity-1' }],
    },
    {
      name: 'syncOfficialProfileBinding posts explicit sync action',
      method: 'post',
      call: () => syncOfficialProfileBinding('binding-1'),
      args: ['/v2/users/me/official-profile-bindings/binding-1/sync'],
    },
    {
      name: 'deleteOfficialProfileBinding deletes binding without deleting profile',
      method: 'delete',
      call: () => deleteOfficialProfileBinding('binding-1'),
      args: ['/v2/users/me/official-profile-bindings/binding-1'],
    },
    {
      name: 'getAdminIdentityProviders gets provider configuration list',
      method: 'get',
      call: getAdminIdentityProviders,
      args: ['/v2/admin/identity-providers'],
    },
    {
      name: 'getAdminIdentityProvider gets provider by id',
      method: 'get',
      call: () => getAdminIdentityProvider('provider-1'),
      args: ['/v2/admin/identity-providers/provider-1'],
    },
    {
      name: 'createIdentityProvider posts complete provider configuration',
      method: 'post',
      call: () => createIdentityProvider(providerInput),
      args: ['/v2/admin/identity-providers', providerInput],
    },
    {
      name: 'updateIdentityProvider puts one canonical provider resource',
      method: 'put',
      call: () => updateIdentityProvider('provider-1', providerInput),
      args: ['/v2/admin/identity-providers/provider-1', providerInput],
    },
    {
      name: 'deleteIdentityProvider deletes provider by id',
      method: 'delete',
      call: () => deleteIdentityProvider('provider-1'),
      args: ['/v2/admin/identity-providers/provider-1'],
    },
  ]
}
