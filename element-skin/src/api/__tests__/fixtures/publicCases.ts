import {
  getPublicFallbackStatus,
  getPublicHomepageMedia,
  getPublicSettings,
  getPublicSkinLibrary,
} from '../../public'
import type { ApiCase } from './types'

export function publicApiCases(): ApiCase[] {
  return [
    {
      name: 'getPublicSettings gets public settings',
      method: 'get',
      call: getPublicSettings,
      args: ['/v2/public/settings'],
    },
    {
      name: 'getPublicHomepageMedia gets public homepage media',
      method: 'get',
      call: getPublicHomepageMedia,
      args: ['/v2/public/homepage-media'],
    },
    {
      name: 'getPublicFallbackStatus gets fallback status',
      method: 'get',
      call: getPublicFallbackStatus,
      args: ['/v2/public/fallback-status'],
    },
    {
      name: 'getPublicSkinLibrary gets exact library params',
      method: 'get',
      call: () =>
        getPublicSkinLibrary({
          cursor: 'cursor-2',
          limit: 12,
          texture_type: 'skin',
          q: 'blue',
          sort: 'most_used',
        }),
      args: [
        '/v2/public/skin-library',
        {
          params: {
            cursor: 'cursor-2',
            limit: 12,
            texture_type: 'skin',
            q: 'blue',
            sort: 'most_used',
          },
        },
      ],
    },
  ]
}
