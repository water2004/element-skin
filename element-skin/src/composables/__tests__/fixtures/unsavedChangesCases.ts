export const leaveConfirmation = [
  '当前页面有未保存的更改，离开后将丢失。确定放弃并离开吗？',
  '离开页面',
  {
    type: 'warning',
    confirmButtonText: '放弃并离开',
    cancelButtonText: '继续编辑',
    closeOnClickModal: false,
    distinguishCancelAndClose: true,
  },
] as const

export const reloadConfirmation = [
  '重新加载会丢弃当前页面未保存的更改，确定继续吗？',
  '重新加载配置',
  {
    ...leaveConfirmation[2],
    confirmButtonText: '放弃并重新加载',
  },
] as const
