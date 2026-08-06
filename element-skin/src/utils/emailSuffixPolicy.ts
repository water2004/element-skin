import type { PublicEmailSuffixPolicy } from '@/api/types'

export const disabledEmailSuffixPolicy: PublicEmailSuffixPolicy = {
  mode: 'disabled',
  suffixes: [],
}

export function emailSuffixPolicyError(
  email: string,
  policy: PublicEmailSuffixPolicy,
): string | null {
  if (policy.mode === 'disabled') return null
  const normalizedEmail = email.trim().toLowerCase()
  const matches = policy.suffixes.some((suffix) => normalizedEmail.endsWith(suffix.toLowerCase()))
  if (policy.mode === 'allowlist' && !matches) return '请选择本站允许的邮箱后缀'
  if (policy.mode === 'denylist' && matches) return '该邮箱后缀不可用'
  return null
}
