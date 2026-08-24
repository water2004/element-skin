// @vitest-environment node

import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import { isPathInside, resolveStaticAssetRequest } from '../staticAssets'

const backendRoot = fileURLToPath(new URL('../fixtures/skin-backend', import.meta.url))

describe('resolveStaticAssetRequest', () => {
  it.each([
    {
      name: 'root deployment texture',
      base: '/',
      url: '/static/textures/abcdef.png?cache=1',
      type: 'image/png',
      relative: path.join('textures', 'abcdef.png'),
    },
    {
      name: 'subpath deployment carousel image',
      base: '/skin/',
      url: '/skin/static/carousel/home.jpg',
      type: 'image/jpeg',
      relative: path.join('carousel', 'home.jpg'),
    },
  ])('resolves $name exactly', ({ base, url, type, relative }) => {
    const result = resolveStaticAssetRequest(backendRoot, base, url)

    expect(result).toEqual({
      contentType: type,
      filePath: path.resolve(backendRoot, relative),
      rootPath: path.resolve(backendRoot, path.dirname(relative)),
    })
  })

  it.each([
    ['literal parent traversal', '/', '/static/textures/../../config.yaml'],
    ['encoded parent traversal', '/', '/static/textures/%2e%2e/%2e%2e/config.yaml'],
    ['encoded windows separator traversal', '/', '/static/textures/%2e%2e%5cconfig.yaml'],
    ['wrong deployment base', '/skin/', '/static/textures/skin.png'],
    ['base prefix collision', '/skin/', '/skinny/static/textures/skin.png'],
    ['unsupported directory', '/', '/static/private/secret.txt'],
    ['missing filename', '/', '/static/textures/'],
    ['malformed encoding', '/', '/static/textures/%E0%A4%A'],
  ])('rejects %s', (_name, base, url) => {
    expect(resolveStaticAssetRequest(backendRoot, base, url)).toBeNull()
  })
})

describe('isPathInside', () => {
  it('accepts a nested path and rejects the root and sibling-prefix paths', () => {
    const root = path.resolve(backendRoot, 'textures')

    expect(isPathInside(root, path.resolve(root, 'nested/skin.png'))).toBe(true)
    expect(isPathInside(root, root)).toBe(false)
    expect(isPathInside(root, path.resolve(backendRoot, 'textures-secret/key.pem'))).toBe(false)
  })
})
