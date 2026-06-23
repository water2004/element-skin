import { register, resetPassword, sendVerificationCode, siteLogin, siteLogout } from '../../auth'
import type { ApiCase } from './types'

export function authApiCases(): ApiCase[] {
  return [
    {
      name: 'siteLogin posts credentials to /site-login',
      method: 'post',
      call: () => siteLogin({ email: 'user@example.com', password: 'Password123' }),
      args: ['/site-login', { email: 'user@example.com', password: 'Password123' }],
    },
    {
      name: 'register posts account payload to /register',
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
        '/register',
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
      args: ['/send-verification-code', { email: 'user@example.com', type: 'reset' }],
    },
    {
      name: 'resetPassword posts reset payload',
      method: 'post',
      call: () =>
        resetPassword({ email: 'user@example.com', password: 'NewPassword123', code: 'RESET123' }),
      args: [
        '/reset-password',
        { email: 'user@example.com', password: 'NewPassword123', code: 'RESET123' },
      ],
    },
    {
      name: 'siteLogout posts to /site-logout',
      method: 'post',
      call: siteLogout,
      args: ['/site-logout'],
    },
  ]
}
