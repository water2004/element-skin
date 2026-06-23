import {
  clearProfileCape,
  clearProfileSkin,
  createProfile,
  deleteProfile,
  getProfiles,
  patchProfile,
} from '../../profiles'
import type { ApiCase } from './types'

export function profilesApiCases(): ApiCase[] {
  return [
    {
      name: 'getProfiles gets paged profile params',
      method: 'get',
      call: () => getProfiles({ cursor: 'cursor-1', limit: 20 }),
      args: ['/me/profiles', { params: { cursor: 'cursor-1', limit: 20 } }],
    },
    {
      name: 'createProfile posts profile payload',
      method: 'post',
      call: () => createProfile({ name: 'Steve', model: 'slim' }),
      args: ['/me/profiles', { name: 'Steve', model: 'slim' }],
    },
    {
      name: 'patchProfile patches profile by id',
      method: 'patch',
      call: () => patchProfile('profile-1', { name: 'Alex' }),
      args: ['/me/profiles/profile-1', { name: 'Alex' }],
    },
    {
      name: 'deleteProfile deletes profile by id',
      method: 'delete',
      call: () => deleteProfile('profile-1'),
      args: ['/me/profiles/profile-1'],
    },
    {
      name: 'clearProfileSkin deletes profile skin',
      method: 'delete',
      call: () => clearProfileSkin('profile-1'),
      args: ['/me/profiles/profile-1/skin'],
    },
    {
      name: 'clearProfileCape deletes profile cape',
      method: 'delete',
      call: () => clearProfileCape('profile-1'),
      args: ['/me/profiles/profile-1/cape'],
    },
  ]
}
