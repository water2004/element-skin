import type { ApiCaseContext } from './types'

function fileFormData(contents: string, filename: string): FormData {
  const formData = new FormData()
  formData.append('file', new Blob([contents]), filename)
  return formData
}

export function createApiCaseContext(): ApiCaseContext {
  return {
    textureForm: fileFormData('texture', 'texture.png'),
    homepageImageForm: fileFormData('image', 'image.jpg'),
    panoramaForm: fileFormData('panorama', 'panorama.jpg'),
  }
}
