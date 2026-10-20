import { expect, test, type Page } from '@playwright/test'
import type { HeroFrame } from './fixtures/heroScene'
import { imageSvg, nextImageSvg, panoramaSvg, resizeViewports } from './fixtures/heroAssets'

async function step(page: Page, at: number): Promise<HeroFrame> {
  return page.evaluate((time) => window.heroTest.step(time), at)
}

function expectPixel(actual: number[], expected: number[]) {
  expect(actual[3]).toBe(255)
  for (let channel = 0; channel < 3; channel++) {
    expect(Math.abs(actual[channel] - expected[channel])).toBeLessThanOrEqual(1)
  }
}

async function openScene(page: Page, kind: string, theme: string) {
  await page.route('**/static/carousel/**', async (route) => {
    const url = route.request().url()
    const body = url.endsWith('next.svg')
      ? nextImageSvg
      : url.endsWith('image.svg')
        ? imageSvg
        : panoramaSvg
    await route.fulfill({ contentType: 'image/svg+xml', body })
  })
  await page.setViewportSize({ width: 1280, height: 720 })
  await page.goto(`/e2e/fixtures/hero-scene.html?kind=${kind}&theme=${theme}`)
  let time = 0
  const green = theme === 'dark' ? 188 : 255
  await expect.poll(async () => (await step(page, (time += 100))).pixels[1][1]).toBe(green)
  await page.evaluate(() => window.heroTest.reset())
  return step(page, 100)
}

for (const theme of ['light', 'dark']) {
  test(`image stays visible and crops correctly during continuous resize (${theme})`, async ({
    page,
  }, testInfo) => {
    const errors: string[] = []
    page.on('pageerror', (error) => errors.push(error.message))
    const initial = await openScene(page, 'image', theme)
    const value = theme === 'dark' ? 188 : 255
    expectPixel(initial.pixels[0], [value, 0, 0])
    expectPixel(initial.pixels[1], [0, value, 0])
    expectPixel(initial.pixels[2], [0, 0, value])

    for (const [index, viewport] of resizeViewports.entries()) {
      await page.setViewportSize(viewport)
      const frame = await step(page, 101 + index)
      expect([frame.width, frame.height]).toEqual([viewport.width, viewport.height])
      expectPixel(frame.pixels[1], [0, value, 0])
      if (viewport.width === 390) {
        frame.pixels.forEach((pixel) => expectPixel(pixel, [0, value, 0]))
        await page.screenshot({ path: testInfo.outputPath('mobile.png') })
      }
    }
    await page.screenshot({ path: testInfo.outputPath('desktop.png') })
    expect(errors).toEqual([])
  })

  test(`panorama stays visible across desktop and narrow viewports (${theme})`, async ({
    page,
  }) => {
    await openScene(page, 'panorama', theme)
    const value = theme === 'dark' ? 188 : 255
    for (const [index, viewport] of resizeViewports.entries()) {
      await page.setViewportSize(viewport)
      const frame = await step(page, 101 + index)
      expect([frame.width, frame.height]).toEqual([viewport.width, viewport.height])
      frame.pixels.forEach((pixel) => expectPixel(pixel, [0, value, 0]))
    }
  })

  test(`carousel keeps its transition progress through a resize (${theme})`, async ({ page }) => {
    await openScene(page, 'carousel', theme)
    await step(page, 1000)
    await page.setViewportSize({ width: 390, height: 844 })
    const halfway = await step(page, 1500)
    const mixed = theme === 'dark' ? 137 : 188
    expectPixel(halfway.pixels[1], [0, mixed, mixed])
    const finished = await step(page, 2000)
    expectPixel(finished.pixels[1], [0, 0, theme === 'dark' ? 188 : 255])
  })
}
