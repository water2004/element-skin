export interface TextureUploadForm {
  texture_type: string
  model: string
  note: string
  is_public: boolean
  file: File | null
}

export function createDefaultUploadForm(): TextureUploadForm {
  return {
    texture_type: 'skin',
    model: 'default',
    note: '',
    is_public: false,
    file: null,
  }
}

export function isTextureUploadDirty(form: TextureUploadForm): boolean {
  const initial = createDefaultUploadForm()
  return (
    form.file !== null ||
    form.texture_type !== initial.texture_type ||
    form.model !== initial.model ||
    form.note !== initial.note ||
    form.is_public !== initial.is_public
  )
}

export function replaceLocalTextureUrl(current: string | null, file: File | null): string | null {
  if (current) URL.revokeObjectURL(current)
  return file ? URL.createObjectURL(file) : null
}

export function disposeLocalTextureUrl(url: string | null): void {
  if (url) URL.revokeObjectURL(url)
}
