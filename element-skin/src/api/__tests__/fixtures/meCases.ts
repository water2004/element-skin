import { changePassword, deleteMe, getMe, patchMe } from '../../me'
import type { ApiCase } from './types'

export function meApiCases(): ApiCase[] {
  return [
    { name: 'getMe gets /me', method: 'get', call: getMe, args: ['/v1/users/me'] },
    {
      name: 'patchMe patches profile fields',
      method: 'patch',
      call: () => patchMe({ display_name: 'Display', avatar_hash: null }),
      args: ['/v1/users/me', { display_name: 'Display', avatar_hash: null }],
    },
    { name: 'deleteMe deletes /me', method: 'delete', call: deleteMe, args: ['/v1/users/me'] },
    {
      name: 'changePassword posts password payload',
      method: 'post',
      call: () => changePassword({ old_password: 'OldPassword123', new_password: 'NewPassword123' }),
      args: ['/v1/users/me/password', { old_password: 'OldPassword123', new_password: 'NewPassword123' }],
    },
  ]
}
