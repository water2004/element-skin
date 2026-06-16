import client from './client'
import type { FallbackStatusResponse, HomepageMedia, SiteSettings, Texture } from './types'

export function getPublicSettings(): Promise<{ data: SiteSettings }> {
  return client.get('/public/settings')
}

export function getPublicHomepageMedia(): Promise<{ data: HomepageMedia[] }> {
  return client.get('/public/homepage-media')
}

export function getPublicFallbackStatus(): Promise<{ data: FallbackStatusResponse }> {
  return client.get('/public/fallback-status')
}

export function getPublicSkinLibrary(params: {
  cursor?: string | null
  limit?: number
  texture_type?: string
  q?: string
  sort?: 'latest' | 'most_used'
}): Promise<{ data: { items: Texture[]; has_next: boolean; next_cursor: string | null; page_size: number } }> {
  return client.get('/public/skin-library', { params })
}
