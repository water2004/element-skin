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
  deleteUser,
  getUser,
  getUserProfiles,
  getUsers,
  resetUserPassword,
  toggleAdmin,
  transferSuperAdmin,
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
      args: ['/admin/homepage-media'],
    },
    {
      name: 'uploadHomepageImage posts image FormData',
      method: 'post',
      call: () => uploadHomepageImage(context.homepageImageForm),
      args: ['/admin/homepage-media/image', context.homepageImageForm],
    },
    {
      name: 'uploadHomepagePanorama posts panorama FormData',
      method: 'post',
      call: () => uploadHomepagePanorama(context.panoramaForm),
      args: ['/admin/homepage-media/panorama', context.panoramaForm],
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
        '/admin/homepage-media/media-1',
        { title: 'Hero', enabled: true, duration_ms: 5000, overlay_opacity_light: 0.2 },
      ],
    },
    {
      name: 'reorderHomepageMedia patches id order',
      method: 'patch',
      call: () => reorderHomepageMedia(['media-2', 'media-1']),
      args: ['/admin/homepage-media/reorder', { ids: ['media-2', 'media-1'] }],
    },
    {
      name: 'deleteHomepageMedia deletes media id',
      method: 'delete',
      call: () => deleteHomepageMedia('media-1'),
      args: ['/admin/homepage-media/media-1'],
    },
  ]
}

function inviteCases(): ApiCase[] {
  return [
    {
      name: 'getAdminInvites gets invite params',
      method: 'get',
      call: () => getAdminInvites({ cursor: null, limit: 50 }),
      args: ['/admin/invites', { params: { cursor: null, limit: 50 } }],
    },
    {
      name: 'createAdminInvite posts invite payload',
      method: 'post',
      call: () => createAdminInvite({ code: 'WELCOME', total_uses: 10, note: 'Launch' }),
      args: ['/admin/invites', { code: 'WELCOME', total_uses: 10, note: 'Launch' }],
    },
    {
      name: 'deleteAdminInvite deletes invite code',
      method: 'delete',
      call: () => deleteAdminInvite('WELCOME'),
      args: ['/admin/invites/WELCOME'],
    },
  ]
}

function adminProfileCases(): ApiCase[] {
  return [
    {
      name: 'getAdminProfiles gets profile params',
      method: 'get',
      call: () => getAdminProfiles({ cursor: 'admin-profile-cursor', limit: 10, q: 'Alex' }),
      args: ['/admin/profiles', { params: { cursor: 'admin-profile-cursor', limit: 10, q: 'Alex' } }],
    },
    {
      name: 'patchAdminProfile patches admin profile',
      method: 'patch',
      call: () => patchAdminProfile('profile-2', { name: 'AdminAlex' }),
      args: ['/admin/profiles/profile-2', { name: 'AdminAlex' }],
    },
    {
      name: 'deleteAdminProfile deletes admin profile',
      method: 'delete',
      call: () => deleteAdminProfile('profile-2'),
      args: ['/admin/profiles/profile-2'],
    },
    {
      name: 'patchProfileSkin patches admin profile skin hash',
      method: 'patch',
      call: () => patchProfileSkin('profile-2', { hash: 'skin-hash' }),
      args: ['/admin/profiles/profile-2/skin', { hash: 'skin-hash' }],
    },
    {
      name: 'patchProfileCape patches admin profile cape hash',
      method: 'patch',
      call: () => patchProfileCape('profile-2', { hash: null }),
      args: ['/admin/profiles/profile-2/cape', { hash: null }],
    },
  ]
}

function adminSettingsCases(): ApiCase[] {
  return [
    {
      name: 'getAdminSettingsGroup gets named settings group',
      method: 'get',
      call: () => getAdminSettingsGroup('site'),
      args: ['/admin/settings/site'],
    },
    {
      name: 'saveAdminSettingsGroup posts settings group',
      method: 'post',
      call: () => saveAdminSettingsGroup('site', { site_name: 'Element Skin' }),
      args: ['/admin/settings/site', { site_name: 'Element Skin' }],
    },
  ]
}

function adminTextureCases(): ApiCase[] {
  return [
    {
      name: 'getAdminTextures gets admin texture params',
      method: 'get',
      call: () => getAdminTextures({ cursor: 'texture-admin-cursor', limit: 25, q: 'cape', type: 'cape' }),
      args: [
        '/admin/textures',
        { params: { cursor: 'texture-admin-cursor', limit: 25, q: 'cape', type: 'cape' } },
      ],
    },
    {
      name: 'patchAdminTexture patches admin texture',
      method: 'patch',
      call: () => patchAdminTexture('hash-admin', { type: 'skin', model: 'slim', note: 'OK', is_public: 1 }),
      args: ['/admin/textures/hash-admin', { type: 'skin', model: 'slim', note: 'OK', is_public: 1 }],
    },
    {
      name: 'deleteAdminTexture deletes admin texture with params',
      method: 'delete',
      call: () => deleteAdminTexture('hash-admin', { type: 'skin', user_id: 'user-1', force: true }),
      args: ['/admin/textures/hash-admin', { params: { type: 'skin', user_id: 'user-1', force: true } }],
    },
  ]
}

function adminUserCases(): ApiCase[] {
  return [
    {
      name: 'getUsers gets user params',
      method: 'get',
      call: () => getUsers({ cursor: 'user-cursor', limit: 15, q: 'mail' }),
      args: ['/admin/users', { params: { cursor: 'user-cursor', limit: 15, q: 'mail' } }],
    },
    {
      name: 'getUser gets user detail',
      method: 'get',
      call: () => getUser('user-1'),
      args: ['/admin/users/user-1'],
    },
    {
      name: 'getUserProfiles gets user profile params',
      method: 'get',
      call: () => getUserProfiles('user-1', { cursor: 'profiles', limit: 8 }),
      args: ['/admin/users/user-1/profiles', { params: { cursor: 'profiles', limit: 8 } }],
    },
    {
      name: 'toggleAdmin posts toggle endpoint',
      method: 'post',
      call: () => toggleAdmin('user-1'),
      args: ['/admin/users/user-1/toggle-admin'],
    },
    {
      name: 'transferSuperAdmin posts transfer endpoint',
      method: 'post',
      call: () => transferSuperAdmin('user-1'),
      args: ['/admin/users/user-1/transfer-super-admin'],
    },
    {
      name: 'deleteUser deletes user',
      method: 'delete',
      call: () => deleteUser('user-1'),
      args: ['/admin/users/user-1'],
    },
    {
      name: 'banUser posts ban timestamp',
      method: 'post',
      call: () => banUser('user-1', { banned_until: 1_700_000_000_000 }),
      args: ['/admin/users/user-1/ban', { banned_until: 1_700_000_000_000 }],
    },
    {
      name: 'unbanUser posts unban endpoint',
      method: 'post',
      call: () => unbanUser('user-1'),
      args: ['/admin/users/user-1/unban'],
    },
    {
      name: 'resetUserPassword posts reset payload',
      method: 'post',
      call: () => resetUserPassword({ user_id: 'user-1', new_password: 'NewPassword123' }),
      args: ['/admin/users/reset-password', { user_id: 'user-1', new_password: 'NewPassword123' }],
    },
  ]
}

function whitelistCases(): ApiCase[] {
  return [
    {
      name: 'getWhitelist gets endpoint whitelist',
      method: 'get',
      call: () => getWhitelist(7),
      args: ['/admin/official-whitelist', { params: { endpoint_id: 7 } }],
    },
    {
      name: 'addWhitelistUser posts whitelist payload',
      method: 'post',
      call: () => addWhitelistUser({ username: 'Player', endpoint_id: 7 }),
      args: ['/admin/official-whitelist', { username: 'Player', endpoint_id: 7 }],
    },
    {
      name: 'removeWhitelistUser deletes whitelist user with endpoint param',
      method: 'delete',
      call: () => removeWhitelistUser('Player', 7),
      args: ['/admin/official-whitelist/Player', { params: { endpoint_id: 7 } }],
    },
  ]
}
