import client from '../client'
import type { User, Profile, PermissionOverrideEffect, UserPermissionsResponse } from '../types'

export function getUsers(params: { cursor?: string | null; limit?: number; q?: string }): Promise<{
  data: { items: User[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v2/admin/users', { params })
}

export function getUser(userId: string): Promise<{ data: User }> {
  return client.get(`/v2/admin/users/${userId}`)
}

export function getUserProfiles(
  userId: string,
  params: { cursor?: string | null; limit?: number },
): Promise<{
  data: { items: Profile[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get(`/v2/admin/users/${userId}/profiles`, { params })
}

export function getUserPermissions(userId: string): Promise<{ data: UserPermissionsResponse }> {
  return client.get(`/v2/admin/users/${userId}/permissions`)
}

export function grantUserRole(
  userId: string,
  roleId: string,
): Promise<{ data: void }> {
  return client.put(`/v2/admin/users/${userId}/roles/${roleId}`)
}

export function revokeUserRole(
  userId: string,
  roleId: string,
): Promise<{ data: void }> {
  return client.delete(`/v2/admin/users/${userId}/roles/${roleId}`)
}

export function transferProtectedSubject(
  userId: string,
): Promise<{ data: void }> {
  return client.post(`/v2/admin/users/${userId}/protected-subject/transfer`)
}

export function setUserPermissionOverride(
  userId: string,
  permissionCode: string,
  effect: PermissionOverrideEffect,
): Promise<{ data: void }> {
  return client.put(`/v2/admin/users/${userId}/permissions/${permissionCode}`, { effect })
}

export function clearUserPermissionOverride(
  userId: string,
  permissionCode: string,
): Promise<{ data: void }> {
  return client.delete(`/v2/admin/users/${userId}/permissions/${permissionCode}`)
}

export function deleteUser(userId: string): Promise<{ data: void }> {
  return client.delete(`/v2/admin/users/${userId}`)
}

export function banUser(
  userId: string,
  data: { banned_until: number; reason: string },
): Promise<{ data: { banned_until: number } }> {
  return client.post(`/v2/admin/users/${userId}/ban`, data)
}

export function unbanUser(userId: string): Promise<{ data: void }> {
  return client.post(`/v2/admin/users/${userId}/unban`)
}

export function resetUserPassword(data: {
  user_id: string
  new_password: string
}): Promise<{ data: void }> {
  return client.post('/v2/admin/users/password/reset', data)
}
