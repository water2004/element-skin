import { expect, test } from '@playwright/test'
import { mediaFor, settings, user } from '../verification/hero-resize/assets.mjs'

test.use({ baseURL: 'http://127.0.0.1:5174' })

for (const theme of ['light', 'dark']) {
  for (const media of ['image', 'panorama', 'carousel']) {
    test(`portable package supports ${media}, ${theme}, both login states and resizing`, async ({
      page,
    }, testInfo) => {
      const errors: string[] = []
      const externalRequests: string[] = []
      let homepageLoads = 0
      page.on('pageerror', (error) => errors.push(error.message))
      page.on('request', (request) => {
        if (!request.url().startsWith('http://127.0.0.1:5174/')) {
          externalRequests.push(request.url())
        }
        if (new URL(request.url()).pathname === '/v2/public/homepage-media') homepageLoads++
      })
      await page.setViewportSize({ width: 1440, height: 1000 })
      let mediaResponse = page.waitForResponse('**/v2/public/homepage-media')
      await page.goto('/__verify/')
      await mediaResponse
      if (theme !== 'light') {
        mediaResponse = page.waitForResponse('**/v2/public/homepage-media')
        await page.locator('#theme').selectOption(theme)
        await mediaResponse
      }
      if (media !== 'image') {
        mediaResponse = page.waitForResponse('**/v2/public/homepage-media')
        await page.locator('#media').selectOption(media)
      }
      expect(await (await mediaResponse).json()).toEqual({ items: mediaFor(media) })

      const preview = page.frameLocator('#preview')
      await expect(preview.getByRole('button', { name: '登录账号', exact: true })).toBeVisible()
      await expect(preview.getByRole('button', { name: '即刻注册', exact: true })).toBeVisible()
      await expect(preview.locator('html')).toHaveClass(theme === 'dark' ? /dark/ : /^(?!.*dark)/)
      const canvas = preview.locator('canvas.hero-bg-fixed')
      await expect(canvas).toHaveCSS('position', 'fixed')
      const origin = await canvas.evaluate(() => performance.timeOrigin)
      const loadsBeforeResize = homepageLoads
      await page.locator('#mobile').click()
      await expect(page.locator('#size')).toHaveText('390 × 844')
      await expect
        .poll(() => canvas.evaluate((node: HTMLCanvasElement) => [node.width, node.height]))
        .toEqual([390, 844])
      await page.screenshot({ path: testInfo.outputPath('package-mobile.png') })

      await page.locator('#sweep').click()
      await expect(page.locator('#sweep')).toHaveAttribute('aria-pressed', 'true')
      await expect
        .poll(() => canvas.evaluate((node: HTMLCanvasElement) => node.width))
        .not.toBe(390)
      await page.locator('#sweep').click()
      await expect(page.locator('#sweep')).toHaveAttribute('aria-pressed', 'false')
      await page.locator('#desktop').click()
      await expect.poll(() => canvas.evaluate((node: HTMLCanvasElement) => node.width)).toBe(1280)
      expect(await canvas.evaluate(() => performance.timeOrigin)).toBe(origin)
      expect(homepageLoads).toBe(loadsBeforeResize)

      await page.locator('#signedIn').check()
      await expect(preview.getByRole('button', { name: '进入个人面板', exact: true })).toBeVisible()
      await expect(preview.getByRole('button', { name: '登录账号', exact: true })).toHaveCount(0)
      await expect(preview.getByRole('button', { name: '即刻注册', exact: true })).toHaveCount(0)
      await expect(preview.getByRole('button', { name: '进入个人面板', exact: true })).toHaveCSS(
        'position',
        'fixed',
      )
      await expect(page.locator('#direct')).toHaveAttribute(
        'href',
        `/?media=${media}&theme=${theme}&signedIn=1`,
      )
      await page.screenshot({ path: testInfo.outputPath('package-desktop.png') })
      expect(errors).toEqual([])
      expect(externalRequests).toEqual([])
    })
  }
}

test('portable package serves exact fixtures and rejects unsupported operations without state changes', async ({
  request,
}) => {
  const health = await request.get('/__verify/health')
  expect(health.status()).toBe(200)
  expect(await health.json()).toMatchObject({
    status: 'ready',
    service: 'issue-19-verification',
    issue: 19,
    revision: expect.stringMatching(/^[0-9a-f]{40}$/),
  })
  const before = await request.get('/v2/public/settings')
  expect(before.status()).toBe(200)
  expect(await before.json()).toEqual(settings)
  const write = await request.post('/v2/public/settings', { data: { site_name: 'unexpected' } })
  expect(write.status()).toBe(405)
  expect(write.headers().allow).toBe('GET, HEAD')
  expect(await write.json()).toEqual({
    error: { object: 'local_fixture', operation: 'read', reason: 'method_not_allowed' },
  })
  const after = await request.get('/v2/public/settings')
  expect(after.status()).toBe(200)
  expect(await after.json()).toEqual(settings)
  const signedIn = await request.get('/v2/users/me', {
    headers: { referer: 'http://127.0.0.1:5174/?signedIn=1' },
  })
  expect(signedIn.status()).toBe(200)
  expect(await signedIn.json()).toEqual(user)
  const signedOut = await request.get('/v2/users/me')
  expect(signedOut.status()).toBe(401)
  expect(await signedOut.json()).toEqual({
    error: { object: 'session', operation: 'read', reason: 'unauthenticated' },
  })
  const refresh = await request.post('/v2/auth/session/refresh')
  expect(refresh.status()).toBe(401)
  expect(await refresh.json()).toEqual({
    error: { object: 'session', operation: 'refresh', reason: 'unauthenticated' },
  })
  for (const path of [
    '/server.mjs',
    '/assets/..%2f..%2fserver.mjs',
    '/assets/missing.js',
    '/v2/not-configured',
  ]) {
    const response = await request.get(path)
    expect(response.status()).toBe(404)
    expect(await response.json()).toEqual({
      error: { object: 'local_fixture', operation: 'read', reason: 'not_configured' },
    })
  }
  const malformed = await request.get('/assets/%GG.js')
  expect(malformed.status()).toBe(400)
  expect(await malformed.json()).toEqual({
    error: { object: 'local_fixture', operation: 'read', reason: 'invalid_request' },
  })
})
