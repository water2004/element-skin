import type { IdentityProviderAdapter } from '@/api/types'

const identityProviderAdapterLabels: Record<IdentityProviderAdapter, string> = {
  generic_oidc: '通用 OIDC',
  microsoft: 'Microsoft',
  qq: 'QQ 互联',
}

export function identityProviderAdapterLabel(adapter: string): string {
  return identityProviderAdapterLabels[adapter as IdentityProviderAdapter] ?? adapter
}
