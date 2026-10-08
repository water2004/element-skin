import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ getCachedImageUrl: vi.fn() }))
vi.mock('@/storage/renderCache', () => ({
  avatarCacheKey: (hash: string, model: string) => `avatar:${hash}:${model}:256x256`,
  getCachedImageUrl: mocks.getCachedImageUrl,
}))
vi.mock('skinview3d', () => ({}))
vi.mock('@/api/me', () => ({ patchMe: vi.fn() }))

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
})

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
})
