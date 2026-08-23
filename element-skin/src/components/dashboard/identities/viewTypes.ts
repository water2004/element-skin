import type { ExternalIdentity, IdentityProviderAdapter } from '@/api/types'

export interface IdentityProviderGroup {
  id: string
  name: string
  adapter: IdentityProviderAdapter
  icon_url: string
  enabled: boolean
  link_enabled: boolean
  identities: ExternalIdentity[]
}
