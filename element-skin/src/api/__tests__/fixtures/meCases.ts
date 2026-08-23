import {
  changeEmail,
  changePassword,
  deleteMe,
  getMe,
  patchMe,
  sendEmailChangeCode,
} from '../../me'
import type { ApiCase } from './types'

export function meApiCases(): ApiCase[] {
  return [
    { name: 'getMe gets /me', method: 'get', call: getMe, args: ['/v2/users/me'] },
    {
      name: 'patchMe patches display name and avatar fields',
      method: 'patch',
      call: () => patchMe({ display_name: 'Display', avatar_hash: null }),
      args: ['/v2/users/me', { display_name: 'Display', avatar_hash: null }],
    },
    {
      name: 'patchMe sends only preferred language when it is the only changed field',
      method: 'patch',
      call: () => patchMe({ preferred_language: 'zh_CN' }),
      args: ['/v2/users/me', { preferred_language: 'zh_CN' }],
    },
    {
      name: 'patchMe sends only avatar hash when the avatar changes independently',
      method: 'patch',
      call: () => patchMe({ avatar_hash: 'avatar-hash' }),
      args: ['/v2/users/me', { avatar_hash: 'avatar-hash' }],
    },
    { name: 'deleteMe deletes /me', method: 'delete', call: deleteMe, args: ['/v2/users/me'] },
    {
      name: 'changePassword posts password payload',
      method: 'post',
      call: () =>
        changePassword({ old_password: 'OldPassword123', new_password: 'NewPassword123' }),
      args: [
        '/v2/users/me/password',
        { old_password: 'OldPassword123', new_password: 'NewPassword123' },
      ],
    },
    {
      name: 'sendEmailChangeCode posts the new email',
      method: 'post',
      call: () => sendEmailChangeCode({ email: 'new@example.com' }),
      args: ['/v2/users/me/email/verification-code', { email: 'new@example.com' }],
    },
    {
      name: 'changeEmail puts the verified email and code',
      method: 'put',
      call: () => changeEmail({ email: 'new@example.com', code: 'EMAIL123' }),
      args: ['/v2/users/me/email', { email: 'new@example.com', code: 'EMAIL123' }],
    },
  ]
}
