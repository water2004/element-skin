import { describe, expect, it } from 'vitest'

import { emailSuffixPolicyError } from '../emailSuffixPolicy'

describe('emailSuffixPolicyError', () => {
  it('allows every suffix when filtering is disabled', () => {
    expect(
      emailSuffixPolicyError('user@anything.test', { mode: 'disabled', suffixes: [] }),
    ).toBeNull()
  })

  it('matches allowlist suffixes case-insensitively and literally', () => {
    const policy = { mode: 'allowlist' as const, suffixes: ['@example.com'] }

    expect(emailSuffixPolicyError(' User@Example.COM ', policy)).toBeNull()
    expect(emailSuffixPolicyError('user@sub.example.com', policy)).toBe('请选择本站允许的邮箱后缀')
  })

  it('rejects only exact denylist suffix matches', () => {
    const policy = { mode: 'denylist' as const, suffixes: ['@blocked.test'] }

    expect(emailSuffixPolicyError('user@BLOCKED.TEST', policy)).toBe('该邮箱后缀不可用')
    expect(emailSuffixPolicyError('user@sub.blocked.test', policy)).toBeNull()
    expect(emailSuffixPolicyError('user@allowed.test', policy)).toBeNull()
  })
})
