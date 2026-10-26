import { boundProfile, otherProfile } from '@/components/common/__tests__/fixtures/dialogFlows'

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
