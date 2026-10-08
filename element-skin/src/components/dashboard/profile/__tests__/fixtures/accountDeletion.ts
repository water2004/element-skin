export const accountDeletionFailures = [
  {
    error: { object: 'protected_subject', operation: 'delete', reason: 'denied' },
    message: '不允许执行此操作',
  },
  {
    error: { object: 'server', operation: 'handle', reason: 'failed' },
    message: '服务器处理请求失败',
  },
]
