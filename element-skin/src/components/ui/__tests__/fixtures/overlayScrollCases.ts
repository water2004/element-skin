export const classicScrollbar = {
  outerWidth: 100,
  innerWidth: 85,
  viewportHeight: 600,
  contentHeight: 1200,
}

export const scrollLockOverlays = [
  { kind: 'dialog', lockClass: 'el-popup-parent--hidden' },
  { kind: 'drawer', lockClass: 'el-popup-parent--hidden' },
  { kind: 'message-box', lockClass: 'el-message-box-parent--hidden' },
  { kind: 'image-viewer', lockClass: 'el-image-viewer-parent--hidden' },
] as const

export const overlayThemes = ['light', 'dark'] as const
export const nestedScrollOverlays = scrollLockOverlays.filter(
  ({ kind }) => kind === 'message-box' || kind === 'image-viewer',
)
