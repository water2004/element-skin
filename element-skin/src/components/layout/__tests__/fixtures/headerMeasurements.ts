export const headerMeasurements = {
  desktop: { viewport: 1200, header: 1200, brandRight: 180, actionsLeft: 1040, nav: 600 },
  longAccount: { viewport: 1200, header: 1200, brandRight: 180, actionsLeft: 850, nav: 600 },
  longSiteName: { viewport: 1200, header: 1200, brandRight: 340, actionsLeft: 1040, nav: 600 },
  longNavigation: { viewport: 1200, header: 1200, brandRight: 180, actionsLeft: 1040, nav: 900 },
  mobile: { viewport: 360, header: 360, brandRight: 80, actionsLeft: 320, nav: 100 },
  breakpoint: { viewport: 768, header: 768, brandRight: 80, actionsLeft: 680, nav: 300 },
  aboveBreakpoint: { viewport: 769, header: 769, brandRight: 80, actionsLeft: 681, nav: 300 },
  exactGap: { viewport: 1200, header: 1200, brandRight: 288, actionsLeft: 912, nav: 600 },
  overlapsGap: { viewport: 1200, header: 1200, brandRight: 289, actionsLeft: 912, nav: 600 },
} as const

export type HeaderMeasurements = {
  [Key in keyof typeof headerMeasurements.desktop]: number
}

export const viewportCases = [
  { label: 'mobile', measurements: headerMeasurements.mobile, visible: false },
  { label: '768px breakpoint', measurements: headerMeasurements.breakpoint, visible: false },
  {
    label: 'above the breakpoint',
    measurements: headerMeasurements.aboveBreakpoint,
    visible: true,
  },
  { label: 'exact 12px safe gap', measurements: headerMeasurements.exactGap, visible: true },
  { label: 'inside the safe gap', measurements: headerMeasurements.overlapsGap, visible: false },
]
