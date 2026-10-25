import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createDefaultUploadForm,
  disposeLocalTextureUrl,
  replaceLocalTextureUrl,
} from '@/components/dashboard/wardrobe/uploadForm'

describe('createDefaultUploadForm', () => {
  it('returns exact default texture upload fields', () => {
    expect(createDefaultUploadForm()).toEqual({
      texture_type: 'skin',
      model: 'default',
      note: '',
      is_public: false,
      file: null,
    })
  })

  it('returns independent objects for each form reset', () => {
    const first = createDefaultUploadForm()
    const second = createDefaultUploadForm()

    first.note = 'changed'
    first.is_public = true

    expect(second).toEqual({
      texture_type: 'skin',
      model: 'default',
      note: '',
      is_public: false,
      file: null,
    })
  })
})

describe('local texture preview url lifecycle', () => {
  const createObjectURL = vi.fn()
  const revokeObjectURL = vi.fn()
  const firstFile = new File(['a'], 'a.png', { type: 'image/png' })
  const secondFile = new File(['b'], 'b.png', { type: 'image/png' })

  beforeEach(() => {
    vi.resetAllMocks()
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })
    createObjectURL.mockImplementation(
      (file: File) => `blob:${file.name}` as `${string}:${string}`,
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('creates a preview url from the selected file without revoking anything', () => {
    expect(replaceLocalTextureUrl(null, firstFile)).toBe('blob:a.png')
    expect(createObjectURL).toHaveBeenCalledWith(firstFile)
    expect(revokeObjectURL).not.toHaveBeenCalled()
  })

  it('revokes the previous url when replacing or clearing the selection', () => {
    const replaced = replaceLocalTextureUrl('blob:a.png', secondFile)
    expect(replaced).toBe('blob:b.png')
    expect(revokeObjectURL).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenNthCalledWith(1, 'blob:a.png')

    expect(replaceLocalTextureUrl(replaced, null)).toBeNull()
    expect(revokeObjectURL).toHaveBeenNthCalledWith(2, 'blob:b.png')
    expect(createObjectURL).toHaveBeenCalledTimes(1)
  })

  it('disposes an existing url and ignores empty urls', () => {
    disposeLocalTextureUrl('blob:a.png')
    disposeLocalTextureUrl(null)
    expect(revokeObjectURL).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:a.png')
  })
})
