import type { IdentityProviderAdapter } from '@/api/types'

interface IdentityProviderAdapterPresentation {
  label: string
  // Optional richer copy for the create/edit form select; falls back to label.
  optionLabel?: string
}

// Single registry for adapter display copy. Adding an adapter means adding one
// entry here plus its backend counterpart; every label consumer derives from it.
const identityProviderAdapters: Record<IdentityProviderAdapter, IdentityProviderAdapterPresentation> =
  {
    generic_oidc: { label: '通用 OIDC' },
    microsoft: { label: 'Microsoft', optionLabel: 'Microsoft（启用正版能力）' },
    qq: { label: 'QQ 互联' },
  }

export function identityProviderAdapterLabel(adapter: string): string {
  return identityProviderAdapters[adapter as IdentityProviderAdapter]?.label ?? adapter
}

export function identityProviderAdapterOptions(): {
  value: IdentityProviderAdapter
  label: string
}[] {
  return Object.entries(identityProviderAdapters).map(([value, presentation]) => ({
    value: value as IdentityProviderAdapter,
    label: presentation.optionLabel ?? presentation.label,
  }))
}
