import client from './client'
import type { LoginResponse } from './types'

export function siteLogin(data: {
  email: string
  password: string
}): Promise<{ data: LoginResponse }> {
  return client.post('/v2/auth/login', data)
}

export function register(data: {
  email: string
  password: string
  username: string
  invite?: string
  code?: string
}): Promise<{ data: { id: string } }> {
  return client.post('/v2/auth/register', data)
}

export function sendVerificationCode(data: {
  email: string
  type: 'register' | 'reset'
}): Promise<{ data: { ttl: number } }> {
  return client.post('/v2/auth/verification-code', data)
}

export function resetPassword(data: {
  email: string
  password: string
  code: string
}): Promise<{ data: void }> {
  return client.post('/v2/auth/password/reset', data)
}

export function siteLogout(): Promise<{ data: void }> {
  return client.post('/v2/auth/logout')
}
