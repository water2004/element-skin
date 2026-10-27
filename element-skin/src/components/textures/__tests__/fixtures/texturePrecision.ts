export const texturePrecisionCases = [
  { type: 'skin', width: 64, height: 64, scale: 1, badge: null },
  { type: 'skin', width: 64, height: 32, scale: 1, badge: null },
  { type: 'skin', width: 128, height: 128, scale: 2, badge: '128px' },
  { type: 'skin', width: 128, height: 64, scale: 2, badge: '128px' },
  { type: 'cape', width: 64, height: 32, scale: 1, badge: null },
  { type: 'cape', width: 22, height: 17, scale: 1, badge: null },
  { type: 'cape', width: 128, height: 64, scale: 2, badge: '128px' },
  { type: 'cape', width: 44, height: 34, scale: 2, badge: '44px' },
  { type: 'cape', width: 88, height: 68, scale: 4, badge: '88px' },
  { type: 'cape', width: 512, height: 256, scale: 8, badge: '512px' },
  { type: 'skin', width: 64, height: 48, scale: null, badge: null },
  { type: 'cape', width: 64, height: 64, scale: null, badge: null },
] as const
