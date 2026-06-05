import client from './client'
import type { SiteSettings, Texture } from './types'

export function getPublicSettings(): Promise<{ data: SiteSettings }> {
  return client.get('/public/settings')
}

export function getPublicCarousel(): Promise<{ data: string[] }> {
  return client.get('/public/carousel')
}

export function getPublicSkinLibrary(params: {
  cursor?: string | null
  limit?: number
  texture_type?: string
  q?: string
}): Promise<{ data: { items: Texture[]; has_next: boolean; next_cursor: string | null; page_size: number } }> {
  return client.get('/public/skin-library', { params })
}
