import type { OfficialProfileBinding, Profile } from '@/api/types'

export const profile: Profile = {
  id: '0123456789abcdef0123456789abcdef',
  name: 'RemoteSteve',
  model: 'slim',
  skin_hash: null,
  cape_hash: null,
}

export const binding: OfficialProfileBinding = {
  id: 'binding-1',
  identity_id: 'identity-1',
  profile_id: profile.id,
  remote_uuid: profile.id,
  remote_name: 'RemoteSteve',
  remote_skin_url: '',
  remote_cape_url: '',
  remote_skin_model: 'slim',
  created_at: 1000,
  updated_at: 1000,
  last_synced_at: null,
  profile,
  identity: {
    id: 'identity-1',
    label: 'Xbox account',
    provider_id: 'microsoft-provider',
    provider_name: 'Microsoft',
    provider_adapter: 'microsoft',
  },
}
