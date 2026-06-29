import {
  addToWardrobe,
  applyTexture,
  deleteTexture,
  getTextureDetail,
  getTextures,
  patchTexture,
  uploadTexture,
} from '../../textures'
import type { ApiCase, ApiCaseContext } from './types'

export function textureApiCases(context: ApiCaseContext): ApiCase[] {
  return [
    {
      name: 'getTextures gets exact texture params',
      method: 'get',
      call: () => getTextures({ cursor: 'texture-cursor', limit: 24, texture_type: 'cape' }),
      args: ['/v1/users/me/textures', { params: { cursor: 'texture-cursor', limit: 24, texture_type: 'cape' } }],
    },
    {
      name: 'uploadTexture posts FormData',
      method: 'post',
      call: () => uploadTexture(context.textureForm),
      args: ['/v1/users/me/textures', context.textureForm],
    },
    {
      name: 'getTextureDetail gets hash/type detail',
      method: 'get',
      call: () => getTextureDetail('hash-1', 'skin'),
      args: ['/v1/users/me/textures/hash-1/skin'],
    },
    {
      name: 'patchTexture patches texture identity',
      method: 'patch',
      call: () => patchTexture('hash-1', 'skin', { note: 'Updated', model: 'slim', is_public: true }),
      args: ['/v1/users/me/textures/hash-1/skin', { note: 'Updated', model: 'slim', is_public: true }],
    },
    {
      name: 'deleteTexture deletes texture identity',
      method: 'delete',
      call: () => deleteTexture('hash-1', 'cape'),
      args: ['/v1/users/me/textures/hash-1/cape'],
    },
    {
      name: 'addToWardrobe posts null body with texture_type param',
      method: 'post',
      call: () => addToWardrobe('hash-1', 'skin'),
      args: ['/v1/users/me/textures/hash-1/wardrobe', null, { params: { texture_type: 'skin' } }],
    },
    {
      name: 'applyTexture posts profile and type payload',
      method: 'post',
      call: () => applyTexture('hash-1', { profile_id: 'profile-1', texture_type: 'skin' }),
      args: ['/v1/users/me/textures/hash-1/apply', { profile_id: 'profile-1', texture_type: 'skin' }],
    },
  ]
}
