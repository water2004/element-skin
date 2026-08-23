import client from '../client'
import type { HomepageMedia, ItemListResponse } from '../types'

export function listHomepageMedia(): Promise<{ data: ItemListResponse<HomepageMedia> }> {
  return client.get('/v2/admin/homepage-media')
}

export function uploadHomepageImage(formData: FormData): Promise<{ data: HomepageMedia }> {
  return client.post('/v2/admin/homepage-media/image', formData)
}

export function uploadHomepagePanorama(formData: FormData): Promise<{ data: HomepageMedia }> {
  return client.post('/v2/admin/homepage-media/panorama', formData)
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
  return client.patch(`/v2/admin/homepage-media/${id}`, body)
}

export function reorderHomepageMedia(ids: string[]): Promise<{ data: void }> {
  return client.patch('/v2/admin/homepage-media/reorder', { ids })
}

export function deleteHomepageMedia(id: string): Promise<{ data: void }> {
  return client.delete(`/v2/admin/homepage-media/${id}`)
}
