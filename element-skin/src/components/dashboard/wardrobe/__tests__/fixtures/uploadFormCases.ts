import type { TextureUploadForm } from '../../uploadForm'

export const uploadChanges: Array<{
  field: keyof TextureUploadForm
  change: Partial<TextureUploadForm>
}> = [
  { field: 'file', change: { file: new File(['texture'], 'texture.png', { type: 'image/png' }) } },
  { field: 'texture_type', change: { texture_type: 'cape' } },
  { field: 'model', change: { model: 'slim' } },
  { field: 'note', change: { note: 'Texture note' } },
  { field: 'is_public', change: { is_public: true } },
]
