import { describe, expect, it, vi } from 'vitest'

import { validateForm } from '../formValidation'

describe('validateForm', () => {
  it('returns the successful validation result exactly', async () => {
    const validate = vi.fn().mockResolvedValue(true)

    await expect(validateForm({ validate })).resolves.toBe(true)
    expect(validate).toHaveBeenCalledTimes(1)
  })

  it('returns false when the form reports an invalid result', async () => {
    const validate = vi.fn().mockResolvedValue(false)

    await expect(validateForm({ validate })).resolves.toBe(false)
    expect(validate).toHaveBeenCalledTimes(1)
  })

  it('returns false for Element Plus validation rejection without exposing it as an API error', async () => {
    const invalidFields = {
      password: [{ field: 'password', message: '密码至少需要6个字符' }],
    }
    const validate = vi.fn().mockRejectedValue(invalidFields)

    await expect(validateForm({ validate })).resolves.toBe(false)
    expect(validate).toHaveBeenCalledTimes(1)
  })

  it('returns false when the form instance is unavailable', async () => {
    await expect(validateForm(null)).resolves.toBe(false)
  })
})
