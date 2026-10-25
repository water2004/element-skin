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

interface HomepageMediaSaveOptions {
  gateway?: HomepageMediaSaveGateway
  onProgress?: (savedItems: HomepageMedia[]) => void
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
  { gateway = httpGateway, onProgress }: HomepageMediaSaveOptions = {},
): Promise<HomepageMedia[]> {
  const changedItems = changedHomepageMediaItems(items, savedItems)
  const orderChanged = homepageMediaOrderChanged(items, savedItems)
  let acknowledged = cloneHomepageMediaItems(savedItems)

  for (const item of changedItems) {
    const updated = await gateway.patch(item.id, buildHomepageMediaPatch(item))
    Object.assign(item, normalizeHomepageMedia(updated))
    // 字段写入不代表排序已保存；失败后的放弃／重试以各请求实际成功的状态为准。
    acknowledged = acknowledged.map((saved) =>
      saved.id === item.id ? { ...item, sort_order: saved.sort_order } : saved,
    )
    onProgress?.(cloneHomepageMediaItems(acknowledged))
  }
  if (orderChanged) {
    await gateway.reorder(items.map((item) => item.id))
    onProgress?.(cloneHomepageMediaItems(items))
  }

  return cloneHomepageMediaItems(items.map(normalizeHomepageMedia))
}
