export interface ApiErrorDescriptor {
  object: string
  operation: string
  reason: string
  params?: Record<string, unknown>
}

interface ApiErrorLike {
  response?: {
    status?: unknown
    data?: {
      error?: unknown
    }
  }
  message?: unknown
}

const exactMessages: Record<string, string> = {
  'authentication.verify.required': '请先登录',
  'credentials.verify.invalid': '账号或密码错误',
  'access_token.verify.invalid': '访问凭证无效',
  'refresh_token.verify.invalid': '刷新凭证无效',
  'refresh_token.verify.expired': '登录状态已过期，请重新登录',
  'permission.check.denied': '没有执行此操作的权限',
  'permission.check.required': '此操作需要额外权限',
  'rate_limit.check.exceeded': '请求过于频繁，请稍后再试',
  'identity.link.already_exists': '该外部身份已绑定到当前账户',
  'identity.link.conflict': '该外部身份已被其他账户绑定',
  'identity.authorize.required': '该外部身份需要重新授权',
  'identity.authorize.mismatch': '授权账户与已绑定身份不一致',
  'identity.authorize.denied': '外部身份授权已被拒绝',
  'identity.authorize.incomplete': '身份连接未完成，原有身份没有改变',
  'identity_provider.login.disabled': '该身份提供方已禁用登录',
  'identity_provider.link.disabled': '该身份提供方已禁用绑定',
  'identity_provider.resolve.not_found': '身份提供方不存在',
  'official_profile.bind.already_exists': '该正版角色已绑定到当前账户',
  'official_profile.bind.conflict': '该正版角色已被其他账户绑定',
  'official_profile.sync.mismatch': '当前 Microsoft 账户与正版角色绑定不一致',
  'profile_name.reserve.conflict': '角色名已被占用',
  'profile.resolve.not_found': '角色不存在',
  'texture.resolve.not_found': '材质不存在',
  'oauth_client.resolve.not_found': '第三方应用不存在',
  'oauth_grant.resolve.not_found': '应用授权不存在',
  'webhook_event.subscribe.denied': '应用权限不允许订阅该 Webhook 事件',
  'server.handle.failed': '服务器处理请求失败',
}

const reasonMessages: Record<string, string> = {
  required: '缺少必填信息',
  invalid: '提交的内容无效',
  not_found: '请求的资源不存在',
  already_exists: '该记录已存在',
  conflict: '当前状态存在冲突',
  denied: '不允许执行此操作',
  expired: '该凭证或操作已过期',
  disabled: '该功能已停用',
  unavailable: '相关服务暂时不可用',
  failed: '操作失败',
  mismatch: '提交的信息与当前记录不匹配',
  unsupported: '不支持提交的内容',
  incomplete: '提交的信息不完整',
  too_large: '提交的内容过大',
  too_long: '提交的内容过长',
  out_of_range: '提交的数值超出允许范围',
  exhausted: '可用次数已耗尽',
  exceeded: '提交的数量超过限制',
}

const oauthMessages: Record<string, string> = {
  invalid_request: 'OAuth 请求无效',
  invalid_client: 'OAuth 客户端认证失败',
  invalid_grant: 'OAuth 授权无效或已过期',
  invalid_scope: 'OAuth 权限范围无效',
  access_denied: 'OAuth 授权已被拒绝',
  authorization_pending: 'OAuth 授权尚未完成',
  expired_token: 'OAuth 设备码已过期',
  unsupported_grant_type: '不支持该 OAuth 授权类型',
  invalid_token: 'OAuth 访问凭证无效',
}

function isApiErrorLike(error: unknown): error is ApiErrorLike {
  return typeof error === 'object' && error !== null
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function getApiError(error: unknown): ApiErrorDescriptor | null {
  if (!isApiErrorLike(error)) return null
  const value = error.response?.data?.error
  if (!isRecord(value)) return null
  if (
    typeof value.object !== 'string' ||
    typeof value.operation !== 'string' ||
    typeof value.reason !== 'string'
  ) {
    return null
  }
  return {
    object: value.object,
    operation: value.operation,
    reason: value.reason,
    ...(isRecord(value.params) ? { params: value.params } : {}),
  }
}

export function isApiError(
  error: unknown,
  object: string,
  operation: string,
  reason: string,
) {
  const descriptor = getApiError(error)
  return (
    descriptor?.object === object &&
    descriptor.operation === operation &&
    descriptor.reason === reason
  )
}

function passwordPolicyMessage(params: Record<string, unknown> | undefined) {
  const rules = Array.isArray(params?.rules) ? params.rules : []
  const labels: Record<string, string> = {
    min_length: '至少 8 个字符',
    lowercase: '包含小写字母',
    uppercase: '包含大写字母',
    number: '包含数字',
  }
  const requirements = rules
    .filter((rule): rule is string => typeof rule === 'string' && rule in labels)
    .map((rule) => labels[rule])
  return requirements.length > 0 ? `密码需要${requirements.join('、')}` : '密码不符合安全要求'
}

export function getErrorMessage(error: unknown, fallback = '操作失败') {
  if (!isApiErrorLike(error)) return fallback

  const descriptor = getApiError(error)
  if (descriptor) {
    return getApiErrorMessage(descriptor, fallback)
  }

  const oauthError = error.response?.data?.error
  if (typeof oauthError === 'string') return oauthMessages[oauthError] ?? fallback

  const message = error.message
  if (typeof message === 'string' && message.trim()) return message

  return fallback
}

export function getApiErrorMessage(descriptor: ApiErrorDescriptor, fallback = '操作失败') {
  const key = `${descriptor.object}.${descriptor.operation}.${descriptor.reason}`
  if (key === 'password.validate.invalid') return passwordPolicyMessage(descriptor.params)
  return exactMessages[key] ?? reasonMessages[descriptor.reason] ?? fallback
}

export function getErrorStatus(error: unknown) {
  if (!isApiErrorLike(error)) return null
  return typeof error.response?.status === 'number' ? error.response.status : null
}

export function isExternalIdentityReauthorizationRequired(error: unknown) {
  return (
    getErrorStatus(error) === 409 && isApiError(error, 'identity', 'authorize', 'required')
  )
}

export function isValidationError(error: unknown) {
  if (getApiError(error)?.operation === 'validate') return true
  if (!isApiErrorLike(error)) return false
  return typeof error.message === 'string' && error.message.includes('validate')
}
