import type { HomepageMedia } from '@/api/types'
import type { HomepageMediaSaveGateway } from '@/components/admin/homepage/homepageMediaSave'
import type { HomepageMediaPatch } from '@/components/admin/homepage/homepageMediaState'

export type HomepageMediaSaveCall =
  | { method: 'patch'; id: string; body: HomepageMediaPatch }
  | { method: 'reorder'; ids: string[] }

export function createHomepageMedia(overrides: Partial<HomepageMedia> = {}): HomepageMedia {
  return {
    id: 'media-1',
    type: 'image',
    title: 'Village',
    storage_path: 'media-1.png',
    overlay_opacity_light: 0.2,
    overlay_opacity_dark: 0.4,
    start_yaw: 10,
    start_pitch: 5,
    yaw_speed_dps: 1,
    pitch_speed_dps: 2,
    sort_order: 0,
    enabled: true,
    duration_ms: 8000,
    created_at: 100,
    updated_at: 200,
    ...overrides,
  }
}

export function createHomepageMediaSaveGateway(
  responses: Record<string, HomepageMedia>,
  options: { patchFailureId?: string; reorderFailure?: boolean } = {},
): { calls: HomepageMediaSaveCall[]; gateway: HomepageMediaSaveGateway } {
  const calls: HomepageMediaSaveCall[] = []
  return {
    calls,
    gateway: {
      async patch(id, body) {
        calls.push({ method: 'patch', id, body })
        if (id === options.patchFailureId) throw new Error(`patch failed: ${id}`)
        const response = responses[id]
        if (!response) throw new Error(`missing response: ${id}`)
        return { ...response }
      },
      async reorder(ids) {
        calls.push({ method: 'reorder', ids: [...ids] })
        if (options.reorderFailure) throw new Error('reorder failed')
      },
    },
  }
}
