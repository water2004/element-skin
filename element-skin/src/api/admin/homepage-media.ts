import client from '../client'
import type { HomepageMedia } from '../types'

export function listHomepageMedia(): Promise<{ data: HomepageMedia[] }> {
  return client.get('/v1/admin/homepage-media')
}

export function uploadHomepageImage(formData: FormData): Promise<{ data: HomepageMedia }> {
  return client.post('/v1/admin/homepage-media/image', formData)
}

export function uploadHomepagePanorama(formData: FormData): Promise<{ data: HomepageMedia }> {
  return client.post('/v1/admin/homepage-media/panorama', formData)
}

export function patchHomepageMedia(
  id: string,
  body: Partial<
    Pick<
      HomepageMedia,
      | 'title'
      | 'enabled'
      | 'duration_ms'
      | 'overlay_opacity_light'
      | 'overlay_opacity_dark'
      | 'start_yaw'
      | 'start_pitch'
      | 'yaw_speed_dps'
      | 'pitch_speed_dps'
    >
  >,
): Promise<{ data: HomepageMedia }> {
  return client.patch(`/v1/admin/homepage-media/${id}`, body)
}

export function reorderHomepageMedia(ids: string[]): Promise<{ data: { ok: boolean } }> {
  return client.patch('/v1/admin/homepage-media/reorder', { ids })
}

export function deleteHomepageMedia(id: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/v1/admin/homepage-media/${id}`)
}
