import client from '../client'
import type { User, Profile } from '../types'

export function getUsers(params: { cursor?: string | null; limit?: number; q?: string }): Promise<{
  data: { items: User[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/admin/users', { params })
}

export function getUser(userId: string): Promise<{ data: User }> {
  return client.get(`/admin/users/${userId}`)
}

export function getUserProfiles(userId: string, params: { cursor?: string | null; limit?: number }): Promise<{
  data: { items: Profile[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get(`/admin/users/${userId}/profiles`, { params })
}

export function toggleAdmin(userId: string): Promise<{ data: { ok: boolean } }> {
  return client.post(`/admin/users/${userId}/toggle-admin`)
}

export function deleteUser(userId: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/admin/users/${userId}`)
}

export function banUser(userId: string, data: { banned_until: number }): Promise<{ data: { ok: boolean; banned_until: number } }> {
  return client.post(`/admin/users/${userId}/ban`, data)
}

export function unbanUser(userId: string): Promise<{ data: { ok: boolean } }> {
  return client.post(`/admin/users/${userId}/unban`)
}

export function resetUserPassword(data: { user_id: string; new_password: string }): Promise<{ data: { ok: boolean } }> {
  return client.post('/admin/users/reset-password', data)
}
