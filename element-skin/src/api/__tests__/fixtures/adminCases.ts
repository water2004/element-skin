import {
  deleteHomepageMedia,
  listHomepageMedia,
  patchHomepageMedia,
  reorderHomepageMedia,
  uploadHomepageImage,
  uploadHomepagePanorama,
} from '../../admin/homepage-media'
import { createAdminInvite, deleteAdminInvite, getAdminInvites } from '../../admin/invites'
import {
  createAdminNotice,
  deleteAdminNotice,
  getAdminNotices,
  patchAdminNotice,
} from '../../admin/notices'
import {
  deleteAdminProfile,
  getAdminProfiles,
  patchAdminProfile,
  patchProfileCape,
  patchProfileSkin,
} from '../../admin/profiles'
import { getAdminSettingsGroup, saveAdminSettingsGroup } from '../../admin/settings'
import { deleteAdminTexture, getAdminTextures, patchAdminTexture } from '../../admin/textures'
import {
  banUser,
  clearUserPermissionOverride,
  deleteUser,
  getUserPermissions,
  getUser,
  getUserProfiles,
  getUsers,
  grantUserRole,
  resetUserPassword,
  revokeUserRole,
  setUserPermissionOverride,
  transferProtectedSubject,
  unbanUser,
} from '../../admin/users'
import { addWhitelistUser, getWhitelist, removeWhitelistUser } from '../../admin/whitelist'
import type { ApiCase, ApiCaseContext } from './types'

export function adminApiCases(context: ApiCaseContext): ApiCase[] {
  return [
    ...homepageMediaCases(context),
    ...inviteCases(),
    ...adminProfileCases(),
    ...adminSettingsCases(),
    ...adminNoticeCases(),
    ...adminTextureCases(),
    ...adminUserCases(),
    ...whitelistCases(),
  ]
}

function homepageMediaCases(context: ApiCaseContext): ApiCase[] {
  return [
    {
      name: 'listHomepageMedia gets admin media list',
      method: 'get',
      call: listHomepageMedia,
      args: ['/v1/admin/homepage-media'],
    },
    {
      name: 'uploadHomepageImage posts image FormData',
      method: 'post',
      call: () => uploadHomepageImage(context.homepageImageForm),
      args: ['/v1/admin/homepage-media/image', context.homepageImageForm],
    },
    {
      name: 'uploadHomepagePanorama posts panorama FormData',
      method: 'post',
      call: () => uploadHomepagePanorama(context.panoramaForm),
      args: ['/v1/admin/homepage-media/panorama', context.panoramaForm],
    },
    {
      name: 'patchHomepageMedia patches selected media fields',
      method: 'patch',
      call: () =>
        patchHomepageMedia('media-1', {
          title: 'Hero',
          enabled: true,
          duration_ms: 5000,
          overlay_opacity_light: 0.2,
        }),
      args: [
        '/v1/admin/homepage-media/media-1',
        { title: 'Hero', enabled: true, duration_ms: 5000, overlay_opacity_light: 0.2 },
      ],
    },
    {
      name: 'reorderHomepageMedia patches id order',
      method: 'patch',
      call: () => reorderHomepageMedia(['media-2', 'media-1']),
      args: ['/v1/admin/homepage-media/reorder', { ids: ['media-2', 'media-1'] }],
    },
    {
      name: 'deleteHomepageMedia deletes media id',
      method: 'delete',
      call: () => deleteHomepageMedia('media-1'),
      args: ['/v1/admin/homepage-media/media-1'],
    },
  ]
}

function adminNoticeCases(): ApiCase[] {
  return [
    {
      name: 'getAdminNotices gets notice params',
      method: 'get',
      call: () =>
        getAdminNotices({
          cursor: 'admin-notice-cursor',
          limit: 12,
          type: 'announcement',
          status: 'enabled',
        }),
      args: [
        '/v1/admin/notifications',
        {
          params: {
            cursor: 'admin-notice-cursor',
            limit: 12,
            type: 'announcement',
            status: 'enabled',
          },
        },
      ],
    },
    {
      name: 'createAdminNotice posts notice draft',
      method: 'post',
      call: () =>
        createAdminNotice({
          title: 'OAuth Applications',
          summary: 'OAuth app registration is open.',
          content_markdown: 'Visit **developer settings**.',
          display_mode: 'detail',
          level: 'info',
          audience: 'users',
          enabled: true,
          pinned: true,
          dismissible: true,
          link_text: 'Open',
          link_url: '/oauth/apps',
          starts_at: null,
          ends_at: 1_800_000_000_000,
        }),
      args: [
        '/v1/admin/notifications',
        {
          title: 'OAuth Applications',
          summary: 'OAuth app registration is open.',
          content_markdown: 'Visit **developer settings**.',
          display_mode: 'detail',
          level: 'info',
          audience: 'users',
          enabled: true,
          pinned: true,
          dismissible: true,
          link_text: 'Open',
          link_url: '/oauth/apps',
          starts_at: null,
          ends_at: 1_800_000_000_000,
        },
      ],
    },
    {
      name: 'patchAdminNotice patches selected notice fields',
      method: 'patch',
      call: () => patchAdminNotice('notice-1', { enabled: false, ends_at: null }),
      args: ['/v1/admin/notifications/notice-1', { enabled: false, ends_at: null }],
    },
    {
      name: 'patchAdminNotice sends only enabled for a list toggle',
      method: 'patch',
      call: () => patchAdminNotice('notice-toggle', { enabled: true }),
      args: ['/v1/admin/notifications/notice-toggle', { enabled: true }],
    },
    {
      name: 'deleteAdminNotice deletes notice id',
      method: 'delete',
      call: () => deleteAdminNotice('notice-1'),
      args: ['/v1/admin/notifications/notice-1'],
    },
  ]
}

function inviteCases(): ApiCase[] {
  return [
    {
      name: 'getAdminInvites gets invite params',
      method: 'get',
      call: () => getAdminInvites({ cursor: null, limit: 50 }),
      args: ['/v1/admin/invites', { params: { cursor: null, limit: 50 } }],
    },
    {
      name: 'createAdminInvite posts invite payload',
      method: 'post',
      call: () => createAdminInvite({ code: 'WELCOME', total_uses: 10, note: 'Launch' }),
      args: ['/v1/admin/invites', { code: 'WELCOME', total_uses: 10, note: 'Launch' }],
    },
    {
      name: 'createAdminInvite preserves explicit unlimited usage',
      method: 'post',
      call: () => createAdminInvite({ code: 'UNLIMITED', total_uses: null, note: 'No limit' }),
      args: ['/v1/admin/invites', { code: 'UNLIMITED', total_uses: null, note: 'No limit' }],
    },
    {
      name: 'deleteAdminInvite deletes invite code',
      method: 'delete',
      call: () => deleteAdminInvite('WELCOME'),
      args: ['/v1/admin/invites/WELCOME'],
    },
  ]
}

function adminProfileCases(): ApiCase[] {
  return [
    {
      name: 'getAdminProfiles gets profile params',
      method: 'get',
      call: () => getAdminProfiles({ cursor: 'admin-profile-cursor', limit: 10, q: 'Alex' }),
      args: [
        '/v1/admin/profiles',
        { params: { cursor: 'admin-profile-cursor', limit: 10, q: 'Alex' } },
      ],
    },
    {
      name: 'patchAdminProfile patches admin profile',
      method: 'patch',
      call: () => patchAdminProfile('profile-2', { name: 'AdminAlex' }),
      args: ['/v1/admin/profiles/profile-2', { name: 'AdminAlex' }],
    },
    {
      name: 'deleteAdminProfile deletes admin profile',
      method: 'delete',
      call: () => deleteAdminProfile('profile-2'),
      args: ['/v1/admin/profiles/profile-2'],
    },
    {
      name: 'patchProfileSkin patches admin profile skin hash',
      method: 'patch',
      call: () => patchProfileSkin('profile-2', { hash: 'skin-hash' }),
      args: ['/v1/admin/profiles/profile-2/skin', { hash: 'skin-hash' }],
    },
    {
      name: 'patchProfileCape patches admin profile cape hash',
      method: 'patch',
      call: () => patchProfileCape('profile-2', { hash: null }),
      args: ['/v1/admin/profiles/profile-2/cape', { hash: null }],
    },
  ]
}

function adminSettingsCases(): ApiCase[] {
  const fallbackPayload = {
    fallback_strategy: 'parallel',
    fallback_probe_interval: 1200,
    fallbacks: [
      {
        id: 7,
        priority: 2,
        session_url: 'https://fallback.example/session',
        account_url: 'https://fallback.example/account',
        services_url: 'https://fallback.example/services',
        cache_ttl: 300,
        enable_profile: true,
        enable_hasjoined: false,
        enable_whitelist: true,
        note: 'secondary',
        skin_domains: ['fallback.example', 'cdn.fallback.example'],
      },
    ],
  }
  return [
    {
      name: 'getAdminSettingsGroup gets named settings group',
      method: 'get',
      call: () => getAdminSettingsGroup('site'),
      args: ['/v1/admin/settings/site'],
    },
    {
      name: 'saveAdminSettingsGroup posts settings group',
      method: 'post',
      call: () => saveAdminSettingsGroup('site', { site_name: 'Element Skin' }),
      args: ['/v1/admin/settings/site', { site_name: 'Element Skin' }],
    },
    {
      name: 'saveAdminSettingsGroup posts exact fallback scheduling and endpoint payload',
      method: 'post',
      call: () => saveAdminSettingsGroup('fallback', fallbackPayload),
      args: ['/v1/admin/settings/fallback', fallbackPayload],
    },
  ]
}

function adminTextureCases(): ApiCase[] {
  return [
    {
      name: 'getAdminTextures gets admin texture params',
      method: 'get',
      call: () =>
        getAdminTextures({ cursor: 'texture-admin-cursor', limit: 25, q: 'cape', type: 'cape' }),
      args: [
        '/v1/admin/textures',
        { params: { cursor: 'texture-admin-cursor', limit: 25, q: 'cape', type: 'cape' } },
      ],
    },
    {
      name: 'patchAdminTexture patches every editable texture field',
      method: 'patch',
      call: () =>
        patchAdminTexture('hash-admin', { type: 'skin', model: 'slim', note: 'OK', is_public: 1 }),
      args: [
        '/v1/admin/textures/hash-admin',
        { type: 'skin', model: 'slim', note: 'OK', is_public: 1 },
      ],
    },
    {
      name: 'patchAdminTexture sends only note for an independent note edit',
      method: 'patch',
      call: () => patchAdminTexture('hash-admin-note', { type: 'skin', note: 'Note only' }),
      args: ['/v1/admin/textures/hash-admin-note', { type: 'skin', note: 'Note only' }],
    },
    {
      name: 'patchAdminTexture sends only model for an independent model edit',
      method: 'patch',
      call: () => patchAdminTexture('hash-admin-model', { type: 'skin', model: 'slim' }),
      args: ['/v1/admin/textures/hash-admin-model', { type: 'skin', model: 'slim' }],
    },
    {
      name: 'patchAdminTexture sends only visibility for an independent visibility edit',
      method: 'patch',
      call: () => patchAdminTexture('hash-admin-public', { type: 'cape', is_public: false }),
      args: ['/v1/admin/textures/hash-admin-public', { type: 'cape', is_public: false }],
    },
    {
      name: 'deleteAdminTexture deletes admin texture with params',
      method: 'delete',
      call: () =>
        deleteAdminTexture('hash-admin', { type: 'skin', user_id: 'user-1', force: true }),
      args: [
        '/v1/admin/textures/hash-admin',
        { params: { type: 'skin', user_id: 'user-1', force: true } },
      ],
    },
  ]
}

function adminUserCases(): ApiCase[] {
  return [
    {
      name: 'getUsers gets user params',
      method: 'get',
      call: () => getUsers({ cursor: 'user-cursor', limit: 15, q: 'mail' }),
      args: ['/v1/admin/users', { params: { cursor: 'user-cursor', limit: 15, q: 'mail' } }],
    },
    {
      name: 'getUser gets user detail',
      method: 'get',
      call: () => getUser('user-1'),
      args: ['/v1/admin/users/user-1'],
    },
    {
      name: 'getUserProfiles gets user profile params',
      method: 'get',
      call: () => getUserProfiles('user-1', { cursor: 'profiles', limit: 8 }),
      args: ['/v1/admin/users/user-1/profiles', { params: { cursor: 'profiles', limit: 8 } }],
    },
    {
      name: 'getUserPermissions gets user permission state',
      method: 'get',
      call: () => getUserPermissions('user-1'),
      args: ['/v1/admin/users/user-1/permissions'],
    },
    {
      name: 'grantUserRole puts role assignment',
      method: 'put',
      call: () => grantUserRole('user-1', 'admin'),
      args: ['/v1/admin/users/user-1/roles/admin'],
    },
    {
      name: 'revokeUserRole deletes role assignment',
      method: 'delete',
      call: () => revokeUserRole('user-1', 'admin'),
      args: ['/v1/admin/users/user-1/roles/admin'],
    },
    {
      name: 'transferProtectedSubject posts transfer endpoint',
      method: 'post',
      call: () => transferProtectedSubject('user-1'),
      args: ['/v1/admin/users/user-1/protected-subject/transfer'],
    },
    {
      name: 'setUserPermissionOverride puts permission effect',
      method: 'put',
      call: () => setUserPermissionOverride('user-1', 'notice.create.any', 'allow'),
      args: ['/v1/admin/users/user-1/permissions/notice.create.any', { effect: 'allow' }],
    },
    {
      name: 'clearUserPermissionOverride deletes permission effect',
      method: 'delete',
      call: () => clearUserPermissionOverride('user-1', 'notice.create.any'),
      args: ['/v1/admin/users/user-1/permissions/notice.create.any'],
    },
    {
      name: 'deleteUser deletes user',
      method: 'delete',
      call: () => deleteUser('user-1'),
      args: ['/v1/admin/users/user-1'],
    },
    {
      name: 'banUser posts ban timestamp and reason',
      method: 'post',
      call: () => banUser('user-1', { banned_until: 1_700_000_000_000, reason: 'abuse' }),
      args: ['/v1/admin/users/user-1/ban', { banned_until: 1_700_000_000_000, reason: 'abuse' }],
    },
    {
      name: 'unbanUser posts unban endpoint',
      method: 'post',
      call: () => unbanUser('user-1'),
      args: ['/v1/admin/users/user-1/unban'],
    },
    {
      name: 'resetUserPassword posts reset payload',
      method: 'post',
      call: () => resetUserPassword({ user_id: 'user-1', new_password: 'NewPassword123' }),
      args: [
        '/v1/admin/users/password/reset',
        { user_id: 'user-1', new_password: 'NewPassword123' },
      ],
    },
  ]
}

function whitelistCases(): ApiCase[] {
  return [
    {
      name: 'getWhitelist gets endpoint whitelist',
      method: 'get',
      call: () => getWhitelist(7),
      args: ['/v1/admin/official-whitelist', { params: { endpoint_id: 7 } }],
    },
    {
      name: 'addWhitelistUser posts whitelist payload',
      method: 'post',
      call: () => addWhitelistUser({ username: 'Player', endpoint_id: 7 }),
      args: ['/v1/admin/official-whitelist', { username: 'Player', endpoint_id: 7 }],
    },
    {
      name: 'removeWhitelistUser deletes whitelist user with endpoint param',
      method: 'delete',
      call: () => removeWhitelistUser('Player', 7),
      args: ['/v1/admin/official-whitelist/Player', { params: { endpoint_id: 7 } }],
    },
  ]
}
