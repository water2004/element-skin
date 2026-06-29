import client from './client'
import type { LoginResponse } from './types'

export function siteLogin(data: { email: string; password: string }): Promise<{ data: LoginResponse }> {
  return client.post('/v1/auth/login', data)
}

export function register(data: {
  email: string
  password: string
  username: string
  invite?: string
  code?: string
}): Promise<{ data: { id: string } }> {
  return client.post('/v1/auth/register', data)
}

export function sendVerificationCode(data: { email: string; type: 'register' | 'reset' }): Promise<{ data: { ok: boolean; ttl: number } }> {
  return client.post('/v1/auth/verification-code', data)
}

export function resetPassword(data: { email: string; password: string; code: string }): Promise<{ data: { ok: boolean } }> {
  return client.post('/v1/auth/password/reset', data)
}

export function siteLogout(): Promise<{ data: { ok: boolean } }> {
  return client.post('/v1/auth/logout')
}
