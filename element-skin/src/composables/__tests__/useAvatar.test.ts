import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useAvatar } from '../useAvatar'

const mocks = vi.hoisted(() => ({
  cache: vi.fn(),
  store: vi.fn(),
  blob: vi.fn(),
  patchMe: vi.fn(),
  load: vi.fn(),
  render: vi.fn(),
  dispose: vi.fn(),
  construct: vi.fn(),
}))
vi.mock('@/api/me', () => ({ patchMe: mocks.patchMe }))
vi.mock('@/storage/renderCache', () => ({
  avatarCacheKey: (hash: string, model: string) => `avatar:${hash}:${model}:256x256`,
  getCachedImageUrl: mocks.cache,
  setCachedImageUrl: mocks.store,
  canvasToBlob: mocks.blob,
}))
vi.mock('skinview3d', () => ({
  SkinViewer: class {
    playerObject = { skin: {}, cape: { visible: true }, elytra: { visible: true } }
    playerWrapper = { position: { y: 0 }, rotation: { x: 0, y: 0 } }
    loadSkin = mocks.load
    render = mocks.render
    dispose = mocks.dispose
    constructor(options: unknown) {
      mocks.construct(options)
    }
  },
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}

beforeEach(async () => {
  await useAvatar().initializeAvatar(null)
  vi.resetAllMocks()
})
afterEach(async () => {
  await useAvatar().initializeAvatar(null)
  vi.restoreAllMocks()
})

it('marks a configured avatar as loading throughout its asynchronous cache lookup', async () => {
  const cached = deferred<string | null>()
  mocks.cache.mockReturnValue(cached.promise)
  const avatar = useAvatar()
  const pending = avatar.initializeAvatar('head', 'slim')
  expect(avatar.avatarHash.value).toBe('head')
  expect(avatar.isAvatarLoading.value).toBe(true)
  expect(avatar.currentAvatarImg.value).toBeNull()
  cached.resolve('blob:cached-head')
  await pending
  expect(avatar.currentAvatarImg.value).toBe('blob:cached-head')
  expect(avatar.isAvatarLoading.value).toBe(false)
  expect(mocks.cache).toHaveBeenCalledExactlyOnceWith('avatar', 'avatar:head:slim:256x256')
  expect(mocks.construct).not.toHaveBeenCalled()
})

it('clears a removed avatar and ignores its late cache result', async () => {
  const cached = deferred<string | null>()
  mocks.cache.mockReturnValue(cached.promise)
  const avatar = useAvatar()
  const pending = avatar.initializeAvatar('removed-head')
  await avatar.initializeAvatar(null)
  cached.resolve('blob:removed-head')
  await pending
  expect(avatar.avatarHash.value).toBeNull()
  expect(avatar.currentAvatarImg.value).toBeNull()
  expect(avatar.isAvatarLoading.value).toBe(false)
})

it.each(['older-first', 'newer-first'])(
  'retains the latest avatar when cache responses finish %s',
  async (order) => {
    const older = deferred<string | null>()
    const newer = deferred<string | null>()
    mocks.cache.mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise)
    const avatar = useAvatar()
    const oldRequest = avatar.initializeAvatar('old-head')
    const newRequest = avatar.initializeAvatar('new-head')
    if (order === 'older-first') {
      older.resolve('blob:old-head')
      await oldRequest
      expect(avatar.currentAvatarImg.value).toBeNull()
      expect(avatar.isAvatarLoading.value).toBe(true)
      newer.resolve('blob:new-head')
    } else {
      newer.resolve('blob:new-head')
      await newRequest
      expect(avatar.currentAvatarImg.value).toBe('blob:new-head')
      expect(avatar.isAvatarLoading.value).toBe(false)
      older.resolve('blob:old-head')
    }
    await Promise.all([oldRequest, newRequest])
    expect(avatar.avatarHash.value).toBe('new-head')
    expect(avatar.currentAvatarImg.value).toBe('blob:new-head')
    expect(avatar.isAvatarLoading.value).toBe(false)
  },
)

it('finishes a failed cache read without retaining an old image or a pending state', async () => {
  mocks.cache.mockResolvedValueOnce('blob:old-head')
  const avatar = useAvatar()
  await avatar.initializeAvatar('old-head')
  mocks.cache.mockRejectedValueOnce(new Error('cache unavailable'))
  await avatar.initializeAvatar('new-head')
  expect(avatar.avatarHash.value).toBe('new-head')
  expect(avatar.currentAvatarImg.value).toBeNull()
  expect(avatar.isAvatarLoading.value).toBe(false)
  expect(mocks.construct).not.toHaveBeenCalled()
  expect(mocks.store).not.toHaveBeenCalled()
})

it('keeps loading until an uncached head is rendered and stored', async () => {
  mocks.cache.mockResolvedValue(null)
  const loaded = deferred<void>()
  mocks.load.mockReturnValue(loaded.promise)
  const blob = new Blob(['head'], { type: 'image/png' })
  mocks.blob.mockResolvedValue(blob)
  mocks.store.mockResolvedValue('blob:generated-head')
  const avatar = useAvatar()
  const pending = avatar.initializeAvatar('head')
  await vi.waitFor(() =>
    expect(mocks.load).toHaveBeenCalledExactlyOnceWith('/static/textures/head.png'),
  )
  expect(avatar.isAvatarLoading.value).toBe(true)
  expect(avatar.currentAvatarImg.value).toBeNull()
  loaded.resolve()
  await pending
  expect(avatar.currentAvatarImg.value).toBe('blob:generated-head')
  expect(avatar.isAvatarLoading.value).toBe(false)
  expect(mocks.render).toHaveBeenCalledTimes(1)
  expect(mocks.store).toHaveBeenCalledExactlyOnceWith('avatar', 'avatar:head:default:256x256', blob)
  expect(mocks.dispose).toHaveBeenCalledTimes(1)
})

it('releases the renderer on texture failure without storing or publishing an image', async () => {
  mocks.cache.mockResolvedValue(null)
  mocks.load.mockRejectedValue(new Error('missing skin'))
  vi.spyOn(console, 'error').mockImplementation(() => {})
  const avatar = useAvatar()
  await avatar.initializeAvatar('missing-head')
  expect(avatar.avatarHash.value).toBe('missing-head')
  expect(avatar.currentAvatarImg.value).toBeNull()
  expect(avatar.isAvatarLoading.value).toBe(false)
  expect(mocks.render).not.toHaveBeenCalled()
  expect(mocks.store).not.toHaveBeenCalled()
  expect(mocks.dispose).toHaveBeenCalledTimes(1)
})

it('does not change the current avatar or dispatch a change event when saving fails', async () => {
  mocks.cache.mockResolvedValue('blob:old-head')
  const avatar = useAvatar()
  await avatar.initializeAvatar('old-head')
  const error = new Error('save rejected')
  mocks.patchMe.mockRejectedValue(error)
  const dispatch = vi.spyOn(window, 'dispatchEvent')
  await expect(avatar.setAvatar('new-head')).rejects.toBe(error)
  expect(mocks.patchMe).toHaveBeenCalledExactlyOnceWith({ avatar_hash: 'new-head' })
  expect(avatar.avatarHash.value).toBe('old-head')
  expect(avatar.currentAvatarImg.value).toBe('blob:old-head')
  expect(avatar.isAvatarLoading.value).toBe(false)
  expect(mocks.cache).toHaveBeenCalledTimes(1)
  expect(dispatch).not.toHaveBeenCalled()
})
