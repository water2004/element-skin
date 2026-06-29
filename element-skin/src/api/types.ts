// Generic cursor pagination response (shared across all paginated endpoints)
export interface CursorPageResponse<T> {
  items: T[]
  has_next: boolean
  next_cursor: string | null
  page_size: number
  total?: number
}

// User (returned by GET /v1/users/me, GET /v1/admin/users)
export interface User {
  id: string
  email: string
  display_name?: string
  roles?: string[]
  permissions?: string[]
  avatar_hash?: string | null
  banned_until?: number | null
  profile_count?: number
  texture_count?: number
  lang?: string
  preferred_language?: string
}

// Player profile / game role
export interface Profile {
  id: string
  name: string
  model?: string
  texture_model?: string
  skin_hash?: string | null
  cape_hash?: string | null
  user_id?: string
  owner_email?: string
  owner_display_name?: string
}

// Texture item (wardrobe / skin-library)
export interface Texture {
  hash: string
  type: 'skin' | 'cape'
  model?: string
  note?: string | null
  name?: string | null
  is_public?: number | boolean
  uploader?: string
  uploader_name?: string
  uploader_display_name?: string
  uploader_email?: string
  created_at?: number
  usage_count?: number
}

// Public site settings
export interface SiteSettings {
  site_name?: string
  site_subtitle?: string
  site_url?: string
  api_url?: string
  allow_register?: boolean
  enable_skin_library?: boolean
  email_verify_enabled?: boolean
  footer_text?: string
  filing_icp?: string
  filing_icp_link?: string
  filing_mps?: string
  filing_mps_link?: string
  easter_eggs?: {
    enabled?: string[]
  }
  mojang_status_urls?: Record<string, string>
}

// Auth responses（token 现在通过 HttpOnly cookie 下发，不再出现在 body）
export interface LoginResponse {
  user_id: string
  permissions?: string[]
}

export type PermissionOverrideEffect = 'allow' | 'deny'

export interface PermissionDefinition {
  id: number
  code: string
  description: string
  bit_index: number
  resource: string
  resource_description: string
  action: string
  action_description: string
  scope: string
  scope_description: string
}

export interface PermissionRole {
  id: string
  name: string
  description: string
  system_role: boolean
  protected: boolean
  permissions: string[]
}

export interface UserPermissionOverride {
  permission_code: string
  effect: PermissionOverrideEffect
  created_at: number
}

export interface UserPermissionsResponse {
  roles: string[]
  effective_permissions: string[]
  overrides: UserPermissionOverride[]
  catalog: {
    permissions: PermissionDefinition[]
    roles: PermissionRole[]
  }
}

// Invite code
export interface Invite {
  code: string
  used_count?: number
  total_uses?: number | null
  used_by?: string | null
  note?: string
  created_at?: number
}

// Whitelist entry
export interface WhitelistEntry {
  username: string
  created_at?: number
}

// Microsoft auth
export interface MicrosoftAuthUrlResponse {
  auth_url: string
  state: string
}

export interface MicrosoftGameProfile {
  id: string
  name: string
  has_game?: boolean
}

export interface MicrosoftProfileResponse {
  profile: MicrosoftGameProfile
  has_game: boolean
  import_token: string
}

export interface YggdrasilImportResult {
  items: Array<{ id: string; name: string }>
  success_count: number
  failure_count: number
  failed: Array<{ profile_id: string; profile_name: string; detail: string }>
}

export interface HomepageMedia {
  id: string
  type: 'image' | 'panorama'
  title: string
  storage_path: string
  overlay_opacity_light: number
  overlay_opacity_dark: number
  start_yaw: number
  start_pitch: number
  yaw_speed_dps: number
  pitch_speed_dps: number
  sort_order: number
  enabled: boolean
  duration_ms: number
  created_at: number
  updated_at: number
}

interface FallbackStatusTick {
  checked_at: number
  session: 'up' | 'down'
  account: 'up' | 'down'
  services: 'up' | 'down'
}

export interface FallbackStatusEntry {
  id: number
  priority: number
  note: string
  session_url: string
  account_url: string
  services_url: string
  latest: FallbackStatusTick | null
  history: FallbackStatusTick[]
}

export interface FallbackStatusResponse {
  endpoints: FallbackStatusEntry[]
  retention_ms: number
  generated_at: number
}

export type NoticeType = 'announcement' | 'system' | (string & {})
export type NoticeDisplayMode = 'inline' | 'detail'
export type NoticeLevel = 'info' | 'success' | 'warning' | 'danger'
export type NoticeAudience = 'users' | 'admins'
export type NoticeStatus = 'all' | 'enabled' | 'disabled' | 'scheduled' | 'expired'

export interface Notice {
  id: string
  type: NoticeType
  title: string
  summary: string
  content_markdown: string
  display_mode: NoticeDisplayMode
  level: NoticeLevel
  link_text: string
  link_url: string
  audience: NoticeAudience
  enabled: boolean
  pinned: boolean
  dismissible: boolean
  starts_at: number | null
  ends_at: number | null
  created_by?: string | null
  created_at: number
  updated_at: number
}

export interface NoticeView extends Notice {
  read: boolean
  read_at: number | null
  dismissed_at: number | null
}
