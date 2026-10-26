import { boundProfile, otherProfile } from '@/components/common/__tests__/fixtures/dialogFlows'
import type { Texture } from '@/api/types'

export const contentEntrances = [
  { className: 'animate-fade-in', duration: '320ms' },
  { className: 'animate-card-slide', duration: '400ms' },
] as const

export const cardMotionPage = {
  data: {
    items: [boundProfile, otherProfile],
    has_next: false,
    next_cursor: null,
    page_size: 20,
  },
}

export const textureMotionPage = {
  data: {
    items: [
      { hash: 'motion-skin-a', type: 'skin', name: 'MotionSkinA', model: 'default' },
      { hash: 'motion-skin-b', type: 'skin', name: 'MotionSkinB', model: 'slim' },
    ] satisfies Texture[],
    has_next: false,
    next_cursor: null,
    page_size: 20,
  },
}

export const textureMotionCases = [
  { dark: false, reduced: false },
  { dark: true, reduced: false },
  { dark: false, reduced: true },
  { dark: true, reduced: true },
]
