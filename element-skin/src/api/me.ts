import client from './client'
import type { User } from './types'

export function getMe(): Promise<{ data: User }> {
  return client.get('/me')
}

export function patchMe(data: {
  email?: string
  display_name?: string
  preferred_language?: string
  avatar_hash?: string | null
}): Promise<{ data: { ok: boolean } }> {
  return client.patch('/me', data)
}

export function deleteMe(): Promise<{ data: { ok: boolean } }> {
  return client.delete('/me')
}

export function changePassword(data: { old_password: string; new_password: string }): Promise<{ data: { ok: boolean; message: string } }> {
  return client.post('/me/password', data)
}

export function refreshToken(): Promise<{ data: { is_admin: boolean } }> {
  return client.post('/me/refresh-token')
}
