import client from './client'
import type { Texture } from './types'

export function getTextures(params: {
  cursor?: string | null
  limit?: number
  texture_type?: string
}): Promise<{
  data: { items: Texture[]; has_next: boolean; next_cursor: string | null; page_size: number }
}> {
  return client.get('/v2/users/me/textures', { params })
}

export function uploadTexture(formData: FormData): Promise<{
  data: { hash: string; type: string }
}> {
  return client.post('/v2/users/me/textures', formData)
}

export function getTextureDetail(hash: string, textureType: string): Promise<{ data: Texture }> {
  return client.get(`/v2/users/me/textures/${hash}/${textureType}`)
}

export function patchTexture(
  hash: string,
  textureType: string,
  data: { note?: string; model?: string; is_public?: boolean },
): Promise<{ data: Texture }> {
  return client.patch(`/v2/users/me/textures/${hash}/${textureType}`, data)
}

export function deleteTexture(
  hash: string,
  textureType: string,
): Promise<{ data: void }> {
  return client.delete(`/v2/users/me/textures/${hash}/${textureType}`)
}

export function addToWardrobe(
  hash: string,
  textureType?: string,
): Promise<{ data: void }> {
  return client.post(`/v2/users/me/textures/${hash}/wardrobe`, null, { params: { texture_type: textureType } })
}

export function applyTexture(
  hash: string,
  data: { profile_id: string; texture_type: string },
): Promise<{ data: void }> {
  return client.post(`/v2/users/me/textures/${hash}/apply`, data)
}
