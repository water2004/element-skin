import { patchHomepageMedia, reorderHomepageMedia } from '@/api/admin/homepage-media'
import type { HomepageMedia } from '@/api/types'
import {
  buildHomepageMediaPatch,
  changedHomepageMediaItems,
  cloneHomepageMediaItems,
  homepageMediaOrderChanged,
  normalizeHomepageMedia,
  type HomepageMediaPatch,
} from '@/components/admin/homepage/homepageMediaState'

export interface HomepageMediaSaveGateway {
  patch(id: string, body: HomepageMediaPatch): Promise<HomepageMedia>
  reorder(ids: string[]): Promise<void>
}

const httpGateway: HomepageMediaSaveGateway = {
  async patch(id, body) {
    const response = await patchHomepageMedia(id, body)
    return response.data
  },
  async reorder(ids) {
    await reorderHomepageMedia(ids)
  },
}

export async function saveHomepageMediaChanges(
  items: HomepageMedia[],
  savedItems: HomepageMedia[],
  gateway: HomepageMediaSaveGateway = httpGateway,
): Promise<HomepageMedia[]> {
  const changedItems = changedHomepageMediaItems(items, savedItems)
  const orderChanged = homepageMediaOrderChanged(items, savedItems)

  for (const item of changedItems) {
    const updated = await gateway.patch(item.id, buildHomepageMediaPatch(item))
    Object.assign(item, normalizeHomepageMedia(updated))
  }
  if (orderChanged) {
    await gateway.reorder(items.map((item) => item.id))
  }

  return cloneHomepageMediaItems(items.map(normalizeHomepageMedia))
}
