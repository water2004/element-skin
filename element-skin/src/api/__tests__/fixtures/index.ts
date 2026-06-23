export type { ApiCase, ApiCaseContext, ApiMethod } from './types'

import { adminApiCases } from './adminCases'
import { authApiCases } from './authCases'
import { createApiCaseContext } from './forms'
import { meApiCases } from './meCases'
import { microsoftApiCases } from './microsoftCases'
import { profilesApiCases } from './profilesCases'
import { publicApiCases } from './publicCases'
import { remoteYggApiCases } from './remoteYggCases'
import { textureApiCases } from './textureCases'
import type { ApiCase } from './types'

export function createApiCases(): ApiCase[] {
  const context = createApiCaseContext()
  return [
    ...authApiCases(),
    ...meApiCases(),
    ...microsoftApiCases(),
    ...profilesApiCases(),
    ...publicApiCases(),
    ...remoteYggApiCases(),
    ...textureApiCases(context),
    ...adminApiCases(context),
  ]
}
