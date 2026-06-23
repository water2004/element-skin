export type ApiMethod = 'get' | 'post' | 'patch' | 'delete'

export interface ApiCase {
  name: string
  method: ApiMethod
  call: () => Promise<unknown>
  args: unknown[]
}

export interface ApiCaseContext {
  textureForm: FormData
  homepageImageForm: FormData
  panoramaForm: FormData
}
