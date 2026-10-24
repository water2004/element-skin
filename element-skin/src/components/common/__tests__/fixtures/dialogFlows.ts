import type { HomepageMedia, Profile } from '@/api/types'

export const emailPolicyResponse = {
  data: { email_suffix_policy: { mode: 'disabled', suffixes: [] } },
} as const
export const resetPasswords = { password: 'TestPassword123!', confirmation: 'TestPassword123!' }
export const boundProfile: Profile = {
  id: 'role-a',
  name: 'PlayerA',
  skin_hash: 'skin-a',
  cape_hash: 'cape-a',
  model: 'default',
}
export const otherProfile: Profile = {
  id: 'role-b',
  name: 'PlayerB',
  skin_hash: 'skin-b',
  cape_hash: null,
  model: 'slim',
}
export const mediaItem: HomepageMedia = {
  id: 'media-a',
  type: 'image',
  title: 'Village',
  storage_path: 'village.png',
  overlay_opacity_light: 0.2,
  overlay_opacity_dark: 0.4,
  start_yaw: 0,
  start_pitch: 0,
  yaw_speed_dps: 1,
  pitch_speed_dps: 0,
  sort_order: 0,
  enabled: true,
  duration_ms: 8000,
  created_at: 1000,
  updated_at: 1000,
}
export const noticeInput = { title: '测试公告', summary: '公告摘要' }
export const expectedMediaPatch = {
  title: 'Edited Village',
  enabled: true,
  duration_ms: 8000,
  overlay_opacity_light: 0.2,
  overlay_opacity_dark: 0.4,
}
export const expectedNoticeDraft = {
  title: '测试公告',
  summary: '公告摘要',
  content_markdown: '',
  display_mode: 'inline',
  level: 'info',
  audience: 'users',
  enabled: true,
  pinned: false,
  dismissible: true,
  link_text: '',
  link_url: '',
  starts_at: null,
  ends_at: null,
} as const
