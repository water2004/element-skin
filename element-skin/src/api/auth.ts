import client from './client'
import type { LoginResponse } from './types'

export function siteLogin(data: { email: string; password: string }): Promise<{ data: LoginResponse }> {
  return client.post('/site-login', data)
}

export function register(data: {
  email: string
  password: string
  username: string
  invite?: string
  code?: string
}): Promise<{ data: { id: string } }> {
  return client.post('/register', data)
}

export function sendVerificationCode(data: { email: string; type: 'register' | 'reset' }): Promise<{ data: { ok: boolean; ttl: number } }> {
  return client.post('/send-verification-code', data)
}

export function resetPassword(data: { email: string; password: string; code: string }): Promise<{ data: { ok: boolean } }> {
  return client.post('/reset-password', data)
}

export function siteLogout(): Promise<{ data: { ok: boolean } }> {
  return client.post('/site-logout')
}
