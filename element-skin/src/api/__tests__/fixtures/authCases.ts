import { register, resetPassword, sendVerificationCode, siteLogin, siteLogout } from '../../auth'
import type { ApiCase } from './types'

export function authApiCases(): ApiCase[] {
  return [
    {
      name: 'siteLogin posts credentials to /v1/auth/login',
      method: 'post',
      call: () => siteLogin({ email: 'user@example.com', password: 'Password123' }),
      args: ['/v1/auth/login', { email: 'user@example.com', password: 'Password123' }],
    },
    {
      name: 'register posts account payload to /v1/auth/register',
      method: 'post',
      call: () =>
        register({
          email: 'new@example.com',
          password: 'Password123',
          username: 'NewUser',
          invite: 'INVITE',
          code: 'CODE1234',
        }),
      args: [
        '/v1/auth/register',
        {
          email: 'new@example.com',
          password: 'Password123',
          username: 'NewUser',
          invite: 'INVITE',
          code: 'CODE1234',
        },
      ],
    },
    {
      name: 'sendVerificationCode posts verification request',
      method: 'post',
      call: () => sendVerificationCode({ email: 'user@example.com', type: 'reset' }),
      args: ['/v1/auth/verification-code', { email: 'user@example.com', type: 'reset' }],
    },
    {
      name: 'resetPassword posts reset payload',
      method: 'post',
      call: () =>
        resetPassword({ email: 'user@example.com', password: 'NewPassword123', code: 'RESET123' }),
      args: [
        '/v1/auth/password/reset',
        { email: 'user@example.com', password: 'NewPassword123', code: 'RESET123' },
      ],
    },
    {
      name: 'siteLogout posts to /v1/auth/logout',
      method: 'post',
      call: siteLogout,
      args: ['/v1/auth/logout'],
    },
  ]
}
