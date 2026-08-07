// Generic cursor pagination response (shared across all paginated endpoints)
export interface CursorPageResponse<T> {
  items: T[]
  has_next: boolean
  next_cursor: string | null
  page_size: number
  total?: number
}

export interface ItemListResponse<T> {
  items: T[]
}

export type EmailSuffixPolicyMode = 'disabled' | 'allowlist' | 'denylist'

export interface EmailSuffixPolicy {
  mode: EmailSuffixPolicyMode
  allowlist: string[]
  denylist: string[]
}

export interface PublicEmailSuffixPolicy {
  mode: EmailSuffixPolicyMode
  suffixes: string[]
}

// User (returned by GET /v2/users/me, GET /v2/admin/users)
export interface User {
  id: string
  email: string
  display_name?: string
  roles?: string[]
  protected?: boolean
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
  require_invite?: boolean
  enable_skin_library?: boolean
  email_verify_enabled?: boolean
  email_suffix_policy?: PublicEmailSuffixPolicy
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
  protected: boolean
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

export interface WhitelistResponse {
  items: WhitelistEntry[]
}

export type IdentityProviderAdapter = 'generic_oidc' | 'microsoft'
export type ExternalIdentityAuthorizationStatus = 'active' | 'reauthorization_required'

export interface IdentityProvider {
  id: string
  name: string
  adapter: IdentityProviderAdapter
  icon_url: string
  login_enabled: boolean
  link_enabled: boolean
  registration_enabled: boolean
}

export interface AdminIdentityProvider extends IdentityProvider {
  issuer_url: string
  authorization_endpoint: string
  token_endpoint: string
  userinfo_endpoint: string
  jwks_uri: string
  client_id: string
  has_client_secret: boolean
  scopes: string[]
  enabled: boolean
  display_order: number
  created_at: number
  updated_at: number
}

export interface ExternalIdentity {
  id: string
  provider_id: string
  provider_name: string
  provider_adapter: IdentityProviderAdapter
  provider_icon_url: string
  provider_enabled: boolean
  provider_link_enabled: boolean
  subject: string
  label: string
  email: string
  email_verified: boolean
  display_name: string
  avatar_url: string
  created_at: number
  updated_at: number
  last_login_at: number | null
  authorization_status: ExternalIdentityAuthorizationStatus
  last_refresh_at: number | null
  last_refresh_error_at: number | null
}

export interface OfficialProfileBinding {
  id: string
  identity_id: string
  profile_id: string
  remote_uuid: string
  remote_name: string
  remote_skin_url: string
  remote_cape_url: string
  remote_skin_model: 'default' | 'slim'
  created_at: number
  updated_at: number
  last_synced_at: number | null
  profile: Profile
  identity: {
    id: string
    label: string
    provider_id: string
    provider_name: string
    provider_adapter: IdentityProviderAdapter
  }
}

export interface YggdrasilImportResult {
  items: Profile[]
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
