import client from './client'
import type { CursorPageResponse, NoticeView } from './types'

export interface NoticeListParams {
  cursor?: string | null
  limit?: number
  type?: string
  include_read?: boolean
  dashboard?: boolean
}

export function getNotices(
  params: NoticeListParams = {},
): Promise<{ data: CursorPageResponse<NoticeView> }> {
  return client.get('/v2/notifications', { params })
}

export function getNotice(id: string): Promise<{ data: NoticeView }> {
  return client.get(`/v2/notifications/${id}`)
}

export function markNoticeRead(id: string): Promise<{ data: void }> {
  return client.post(`/v2/notifications/${id}/read`)
}

export function dismissNotice(id: string): Promise<{ data: void }> {
  return client.post(`/v2/notifications/${id}/dismiss`)
}
