import client from './client'
import type { User } from './types'

export function getMe(): Promise<{ data: User }> {
  return client.get('/v2/users/me')
}

export function patchMe(data: {
  display_name?: string
  preferred_language?: string
  avatar_hash?: string | null
}): Promise<{ data: void }> {
  return client.patch('/v2/users/me', data)
}

export function sendEmailChangeCode(data: {
  email: string
}): Promise<{ data: { ttl: number } }> {
  return client.post('/v2/users/me/email/verification-code', data)
}

export function changeEmail(data: {
  email: string
  code: string
}): Promise<{ data: void }> {
  return client.put('/v2/users/me/email', data)
}

export function deleteMe(): Promise<{ data: void }> {
  return client.delete('/v2/users/me')
}

export function changePassword(data: {
  old_password: string
  new_password: string
}): Promise<{ data: void }> {
  return client.post('/v2/users/me/password', data)
}
