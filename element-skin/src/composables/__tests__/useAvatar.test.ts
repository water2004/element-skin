import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ getCachedImageUrl: vi.fn(), patchMe: vi.fn() }))
vi.mock('@/storage/renderCache', () => ({
  avatarCacheKey: (hash: string, model: string) => `avatar:${hash}:${model}:256x256`,
  getCachedImageUrl: mocks.getCachedImageUrl,
}))
vi.mock('skinview3d', () => ({}))
vi.mock('@/api/me', () => ({ patchMe: mocks.patchMe }))

beforeEach(() => {
  vi.resetModules()
  vi.resetAllMocks()
})

afterEach(() => vi.restoreAllMocks())

describe('avatar session state', () => {
  it('clears the current image and hash when resetting the avatar', async () => {
    mocks.getCachedImageUrl.mockResolvedValue('blob:existing-avatar')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    await avatar.initializeAvatar('existing-hash')
    expect(avatar.currentAvatarImg.value).toBe('blob:existing-avatar')
    expect(avatar.avatarHash.value).toBe('existing-hash')
    expect(mocks.getCachedImageUrl).toHaveBeenCalledExactlyOnceWith(
      'avatar',
      'avatar:existing-hash:default:256x256',
    )
    await avatar.initializeAvatar(null)
    expect(avatar.currentAvatarImg.value).toBeNull()
    expect(avatar.avatarHash.value).toBeNull()
    expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(1)
  })

  it('ignores a cached image that arrives after session cleanup', async () => {
    const pending = Promise.withResolvers<string>()
    mocks.getCachedImageUrl.mockReturnValue(pending.promise)
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    const initializing = avatar.initializeAvatar('old-hash')
    await avatar.initializeAvatar(null)
    pending.resolve('blob:old-avatar')
    await initializing
    expect(avatar.currentAvatarImg.value).toBeNull()
    expect(avatar.avatarHash.value).toBeNull()
    expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(1)
  })

  it('ignores an image lookup completion after the initial cache miss and session cleanup', async () => {
    const pending = Promise.withResolvers<string>()
    mocks.getCachedImageUrl.mockResolvedValueOnce(null).mockReturnValueOnce(pending.promise)
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    const initializing = avatar.initializeAvatar('old-hash')
    await Promise.resolve()
    expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(2)
    await avatar.initializeAvatar(null)
    pending.resolve('blob:late-avatar')
    await initializing
    expect(avatar.currentAvatarImg.value).toBeNull()
    expect(avatar.avatarHash.value).toBeNull()
  })

  it('preserves a new avatar when an older lookup completes later', async () => {
    const pending = Promise.withResolvers<string>()
    mocks.getCachedImageUrl
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValueOnce('blob:new-avatar')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    const initializing = avatar.initializeAvatar('old-hash')
    await avatar.initializeAvatar('new-hash')
    pending.resolve('blob:old-avatar')
    await initializing
    expect(avatar.currentAvatarImg.value).toBe('blob:new-avatar')
    expect(avatar.avatarHash.value).toBe('new-hash')
    expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(2)
    expect(mocks.getCachedImageUrl).toHaveBeenNthCalledWith(
      2,
      'avatar',
      'avatar:new-hash:default:256x256',
    )
  })

  it.each([null, 'new-account-hash'])(
    'ignores a pending update after session cleanup and initialization of %s',
    async (newHash) => {
      const pending = Promise.withResolvers<unknown>()
      mocks.patchMe.mockReturnValue(pending.promise)
      mocks.getCachedImageUrl.mockResolvedValue('blob:new-avatar')
      const dispatch = vi.spyOn(window, 'dispatchEvent')
      const { useAvatar } = await import('../useAvatar')
      const avatar = useAvatar()
      const updating = avatar.setAvatar('old-account-hash')
      await avatar.initializeAvatar(null)
      if (newHash) await avatar.initializeAvatar(newHash)
      pending.resolve({})

      expect(await updating).toBe(false)
      expect(avatar.avatarHash.value).toBe(newHash)
      expect(avatar.currentAvatarImg.value).toBe(newHash ? 'blob:new-avatar' : null)
      expect(mocks.patchMe).toHaveBeenCalledExactlyOnceWith({ avatar_hash: 'old-account-hash' })
      expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(newHash ? 1 : 0)
      expect(dispatch).not.toHaveBeenCalled()
    },
  )

  it('does not publish an update if the session clears while its image is loading', async () => {
    const pending = Promise.withResolvers<string>()
    mocks.patchMe.mockResolvedValue({})
    mocks.getCachedImageUrl.mockReturnValue(pending.promise)
    const dispatch = vi.spyOn(window, 'dispatchEvent')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    const updating = avatar.setAvatar('old-hash')
    await Promise.resolve()
    expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(1)
    await avatar.initializeAvatar(null)
    pending.resolve('blob:old-avatar')

    expect(await updating).toBe(false)
    expect(avatar.avatarHash.value).toBeNull()
    expect(avatar.currentAvatarImg.value).toBeNull()
    expect(dispatch).not.toHaveBeenCalled()
  })

  it('keeps the latest requested avatar when update responses arrive in reverse order', async () => {
    const pending = Promise.withResolvers<unknown>()
    mocks.patchMe.mockReturnValueOnce(pending.promise).mockResolvedValueOnce({})
    mocks.getCachedImageUrl.mockResolvedValue('blob:new-avatar')
    const dispatch = vi.spyOn(window, 'dispatchEvent')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    const older = avatar.setAvatar('old-hash')
    expect(await avatar.setAvatar('new-hash', 'slim')).toBe(true)
    pending.resolve({})

    expect(await older).toBe(false)
    expect(avatar.avatarHash.value).toBe('new-hash')
    expect(avatar.currentAvatarImg.value).toBe('blob:new-avatar')
    expect(mocks.patchMe.mock.calls).toEqual([
      [{ avatar_hash: 'old-hash' }],
      [{ avatar_hash: 'new-hash' }],
    ])
    expect(mocks.getCachedImageUrl).toHaveBeenCalledExactlyOnceWith(
      'avatar',
      'avatar:new-hash:slim:256x256',
    )
    expect(dispatch).toHaveBeenCalledTimes(1)
    expect(dispatch.mock.calls[0]?.[0]).toMatchObject({
      type: 'avatar-changed',
      detail: 'new-hash',
    })
  })

  it('publishes a successful avatar removal exactly once', async () => {
    mocks.getCachedImageUrl.mockResolvedValue('blob:existing-avatar')
    mocks.patchMe.mockResolvedValue({})
    const dispatch = vi.spyOn(window, 'dispatchEvent')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    await avatar.initializeAvatar('existing-hash')

    expect(await avatar.setAvatar(null)).toBe(true)
    expect(avatar.avatarHash.value).toBeNull()
    expect(avatar.currentAvatarImg.value).toBeNull()
    expect(mocks.patchMe).toHaveBeenCalledExactlyOnceWith({ avatar_hash: null })
    expect(dispatch).toHaveBeenCalledTimes(1)
    expect(dispatch.mock.calls[0]?.[0]).toMatchObject({ type: 'avatar-changed', detail: null })
  })

  it('keeps the newer request active when the older response arrives first', async () => {
    const olderResponse = Promise.withResolvers<unknown>()
    const newerResponse = Promise.withResolvers<unknown>()
    mocks.patchMe
      .mockReturnValueOnce(olderResponse.promise)
      .mockReturnValueOnce(newerResponse.promise)
    mocks.getCachedImageUrl.mockResolvedValue('blob:new-avatar')
    const dispatch = vi.spyOn(window, 'dispatchEvent')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    const older = avatar.setAvatar('old-hash')
    const newer = avatar.setAvatar('new-hash')
    olderResponse.resolve({})

    expect(await older).toBe(false)
    expect(mocks.getCachedImageUrl).not.toHaveBeenCalled()
    expect(dispatch).not.toHaveBeenCalled()
    newerResponse.resolve({})
    expect(await newer).toBe(true)
    expect(avatar.avatarHash.value).toBe('new-hash')
    expect(avatar.currentAvatarImg.value).toBe('blob:new-avatar')
    expect(dispatch).toHaveBeenCalledTimes(1)
    expect(dispatch.mock.calls[0]?.[0]).toMatchObject({
      type: 'avatar-changed',
      detail: 'new-hash',
    })
  })

  it('allows an existing avatar lookup to finish when an update request fails', async () => {
    const pending = Promise.withResolvers<string>()
    const failure = new Error('request failed')
    mocks.getCachedImageUrl.mockReturnValue(pending.promise)
    mocks.patchMe.mockRejectedValue(failure)
    const dispatch = vi.spyOn(window, 'dispatchEvent')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    const initializing = avatar.initializeAvatar('existing-hash')
    await expect(avatar.setAvatar('rejected-hash')).rejects.toBe(failure)
    pending.resolve('blob:existing-avatar')
    await initializing

    expect(avatar.avatarHash.value).toBe('existing-hash')
    expect(avatar.currentAvatarImg.value).toBe('blob:existing-avatar')
    expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(1)
    expect(dispatch).not.toHaveBeenCalled()
  })

  it('preserves the avatar and emits nothing if the update request fails', async () => {
    const failure = new Error('request failed')
    mocks.getCachedImageUrl.mockResolvedValue('blob:existing-avatar')
    mocks.patchMe.mockRejectedValue(failure)
    const dispatch = vi.spyOn(window, 'dispatchEvent')
    const { useAvatar } = await import('../useAvatar')
    const avatar = useAvatar()
    await avatar.initializeAvatar('existing-hash')

    await expect(avatar.setAvatar('rejected-hash')).rejects.toBe(failure)
    expect(avatar.avatarHash.value).toBe('existing-hash')
    expect(avatar.currentAvatarImg.value).toBe('blob:existing-avatar')
    expect(mocks.getCachedImageUrl).toHaveBeenCalledTimes(1)
    expect(dispatch).not.toHaveBeenCalled()
  })
})
