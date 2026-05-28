import client from '../client'

export function uploadCarousel(formData: FormData): Promise<{ data: { filename: string } }> {
  return client.post('/admin/carousel', formData)
}

export function deleteCarousel(filename: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/admin/carousel/${filename}`)
}
