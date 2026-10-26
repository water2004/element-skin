import type { HomepageMedia } from '@/api/types'

export const heroImage: HomepageMedia = {
  id: 'image',
  type: 'image',
  title: 'Resize regression image',
  storage_path: 'image.svg',
  overlay_opacity_light: 0.2,
  overlay_opacity_dark: 0.6,
  start_yaw: 0,
  start_pitch: 0,
  yaw_speed_dps: 0,
  pitch_speed_dps: 0,
  sort_order: 0,
  enabled: true,
  duration_ms: 1000,
  created_at: 0,
  updated_at: 0,
}

export const heroPanorama: HomepageMedia = {
  ...heroImage,
  id: 'panorama',
  type: 'panorama',
  storage_path: 'panorama',
  start_yaw: 10,
  start_pitch: 5,
  yaw_speed_dps: 6,
  pitch_speed_dps: 2,
}

export const heroNextImage: HomepageMedia = {
  ...heroImage,
  id: 'next',
  storage_path: 'next.svg',
  overlay_opacity_light: 0.4,
  overlay_opacity_dark: 0.8,
}

export const heroResizeViewports = [
  { width: 1100, height: 740 },
  { width: 900, height: 700 },
  { width: 600, height: 800 },
  { width: 390, height: 844 },
  { width: 420, height: 800 },
  { width: 800, height: 600 },
  { width: 1280, height: 720 },
]
