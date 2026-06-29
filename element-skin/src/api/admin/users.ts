import client from '../client'
import type { User, Profile, PermissionOverrideEffect, UserPermissionsResponse } from '../types'

export function getUsers(params: { cursor?: string | null; limit?: number; q?: string }): Promise<{
  data: { items: User[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v1/admin/users', { params })
}

export function getUser(userId: string): Promise<{ data: User }> {
  return client.get(`/v1/admin/users/${userId}`)
}

export function getUserProfiles(
  userId: string,
  params: { cursor?: string | null; limit?: number },
): Promise<{
  data: { items: Profile[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get(`/v1/admin/users/${userId}/profiles`, { params })
}

export function getUserPermissions(userId: string): Promise<{ data: UserPermissionsResponse }> {
  return client.get(`/v1/admin/users/${userId}/permissions`)
}

export function grantUserRole(
  userId: string,
  roleId: string,
): Promise<{ data: { ok: boolean; role_id: string } }> {
  return client.put(`/v1/admin/users/${userId}/roles/${roleId}`)
}

export function revokeUserRole(
  userId: string,
  roleId: string,
): Promise<{ data: { ok: boolean; role_id: string } }> {
  return client.delete(`/v1/admin/users/${userId}/roles/${roleId}`)
}

export function setUserPermissionOverride(
  userId: string,
  permissionCode: string,
  effect: PermissionOverrideEffect,
): Promise<{ data: { ok: boolean; permission_code: string; effect: PermissionOverrideEffect } }> {
  return client.put(`/v1/admin/users/${userId}/permissions/${permissionCode}`, { effect })
}

export function clearUserPermissionOverride(
  userId: string,
  permissionCode: string,
): Promise<{ data: { ok: boolean; permission_code: string } }> {
  return client.delete(`/v1/admin/users/${userId}/permissions/${permissionCode}`)
}

export function deleteUser(userId: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/admin/users/${userId}`)
}

export function banUser(
  userId: string,
  data: { banned_until: number },
): Promise<{ data: { ok: boolean; banned_until: number } }> {
  return client.post(`/v1/admin/users/${userId}/ban`, data)
}

export function unbanUser(userId: string): Promise<{ data: { ok: boolean } }> {
  return client.post(`/v1/admin/users/${userId}/unban`)
}

export function resetUserPassword(data: {
  user_id: string
  new_password: string
}): Promise<{ data: { ok: boolean } }> {
  return client.post('/v1/admin/users/password/reset', data)
}
