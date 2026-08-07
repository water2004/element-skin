interface ApiErrorLike {
  response?: {
    status?: unknown
    data?: {
      detail?: unknown
      error_description?: unknown
    }
  }
  message?: unknown
}

function isApiErrorLike(error: unknown): error is ApiErrorLike {
  return typeof error === 'object' && error !== null
}

export function getErrorMessage(error: unknown, fallback = '操作失败') {
  if (!isApiErrorLike(error)) return fallback

  const detail = error.response?.data?.detail
  if (typeof detail === 'string' && detail.trim()) return detail

  const oauthDescription = error.response?.data?.error_description
  if (typeof oauthDescription === 'string' && oauthDescription.trim()) return oauthDescription

  const message = error.message
  if (typeof message === 'string' && message.trim()) return message

  return fallback
}

export function getErrorStatus(error: unknown) {
  if (!isApiErrorLike(error)) return null
  return typeof error.response?.status === 'number' ? error.response.status : null
}

export function isExternalIdentityReauthorizationRequired(error: unknown) {
  if (!isApiErrorLike(error)) return false
  return (
    error.response?.status === 409 &&
    error.response?.data?.detail === 'external identity must be reauthorized'
  )
}

export function isValidationError(error: unknown) {
  if (!isApiErrorLike(error)) return false
  return typeof error.message === 'string' && error.message.includes('validate')
}
