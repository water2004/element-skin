import type { HomepageMedia, Profile } from '@/api/types'

export const dialogFlowPermissions = [
  'profile.create.owned',
  'profile.update.any',
  'profile.read.any',
  'texture.create.owned',
]

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
export const uploadFileData = { body: 'test image', name: 'test.png', type: 'image/png' }
export const siteSettingsGroups = {
  site: {
    site_name: 'Test Site',
    site_subtitle: 'Test subtitle',
    require_invite: false,
    allow_register: true,
    enable_skin_library: true,
    profile_uuid_mode: 'random',
    max_texture_size: 1024,
    footer_text: '',
    filing_icp: '',
    filing_icp_link: '',
    filing_mps: '',
    filing_mps_link: '',
  },
  security: {
    rate_limit_enabled: true,
    rate_limit_auth_attempts: 5,
    rate_limit_auth_window: 15,
    enable_strong_password_check: false,
  },
  auth: { jwt_expire_days: 7 },
}

export const adminEmailSettings = {
  email_verify_enabled: true,
  email_verify_ttl: 300,
  smtp_host: 'smtp.example.test',
  smtp_port: '465',
  smtp_user: 'mail@example.test',
  smtp_password: 'server-secret-must-not-be-displayed',
  smtp_ssl: true,
  smtp_sender: 'Test Mail',
}

export const adminEmailPolicy = {
  mode: 'disabled',
  allowlist: ['@base.example.test'],
  denylist: [],
}

export const emailFieldChanges = [
  { label: '启用邮件验证', kind: 'switch' },
  { label: '验证码有效期 (秒)', kind: 'number', value: '600' },
  { label: '服务器地址', kind: 'input', value: 'smtp.changed.test' },
  { label: '端口', kind: 'input', value: '587' },
  { label: '用户名 (通常为邮箱地址)', kind: 'input', value: 'changed@example.test' },
  { label: '密码 / 授权码', kind: 'input', value: 'draft-mail-secret' },
  { label: '使用 SSL/TLS 加密', kind: 'switch' },
  { label: '发件人显示名称', kind: 'input', value: 'Changed Mail' },
] as const

export const emailDraftChanges = {
  smtp_host: 'smtp.changed.test',
  smtp_password: 'draft-mail-secret',
  policySuffix: '@new.example.test',
}
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
