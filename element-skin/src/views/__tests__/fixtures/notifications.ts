import type { CursorPageResponse, NoticeView } from '@/api/types'

export const firstNotice: NoticeView = {
  id: 'notice-a',
  type: 'announcement',
  title: 'First notice',
  summary: 'First summary',
  content_markdown: 'First detail body',
  display_mode: 'detail',
  level: 'info',
  link_text: '',
  link_url: '',
  audience: 'users',
  enabled: true,
  pinned: false,
  dismissible: true,
  starts_at: null,
  ends_at: null,
  created_at: 1000,
  updated_at: 1000,
  read: false,
  read_at: null,
  dismissed_at: null,
}
export const secondNotice: NoticeView = {
  ...firstNotice,
  id: 'notice-b',
  title: 'Second notice',
  summary: 'Second summary',
  content_markdown: 'Second detail body',
}
export const noticesPage: CursorPageResponse<NoticeView> = {
  items: [firstNotice, secondNotice],
  has_next: false,
  next_cursor: null,
  page_size: 2,
}
export const firstDetail: NoticeView = { ...firstNotice, read: true, read_at: 2000 }
export const secondDetail: NoticeView = { ...secondNotice, read: true, read_at: 3000 }
