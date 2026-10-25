import { describe, expect, it, vi } from 'vitest'
import { saveHomepageMediaChanges } from '@/components/admin/homepage/homepageMediaSave'
import {
  createHomepageMedia as media,
  createHomepageMediaSaveGateway,
} from '@/components/admin/homepage/__tests__/fixtures/homepageMediaSaveFixtures'

describe('homepage media incremental save', () => {
  it('advances only successful fields after partial failure and retries only remaining changes', async () => {
    const first = media({ id: 'first', title: 'First', sort_order: 0 })
    const second = media({ id: 'second', title: 'Second', sort_order: 1 })
    const changedSecond = { ...second, title: 'Second changed' }
    const changedFirst = { ...first, title: 'First changed' }
    const items = [changedSecond, changedFirst]
    const secondResponse = { ...changedSecond, updated_at: 700 }
    const firstResponse = { ...changedFirst, updated_at: 800 }
    let acknowledged = [first, second]
    const progress = vi.fn((saved) => {
      acknowledged = saved
    })
    const failed = createHomepageMediaSaveGateway(
      { second: secondResponse },
      { patchFailureId: 'first' },
    )
    await expect(
      saveHomepageMediaChanges(items, acknowledged, {
        gateway: failed.gateway,
        onProgress: progress,
      }),
    ).rejects.toThrow('patch failed: first')
    expect(progress).toHaveBeenCalledExactlyOnceWith([first, secondResponse])
    expect(acknowledged).toEqual([first, secondResponse])
    expect(items).toEqual([secondResponse, changedFirst])
    const retry = createHomepageMediaSaveGateway({ first: firstResponse })
    const result = await saveHomepageMediaChanges(items, acknowledged, {
      gateway: retry.gateway,
      onProgress: progress,
    })
    expect(retry.calls).toEqual([
      {
        method: 'patch',
        id: 'first',
        body: {
          title: 'First changed',
          enabled: true,
          duration_ms: 8000,
          overlay_opacity_light: 0.2,
          overlay_opacity_dark: 0.4,
        },
      },
      { method: 'reorder', ids: ['second', 'first'] },
    ])
    expect(result).toEqual([secondResponse, firstResponse])
    expect(acknowledged).toEqual(result)
    expect(progress).toHaveBeenCalledTimes(3)
  })

  it('keeps acknowledged fields and original order when reorder fails so discard does not undo saved fields', async () => {
    const first = media({ id: 'first', title: 'First', sort_order: 0 })
    const second = media({ id: 'second', title: 'Second', sort_order: 1 })
    const response = { ...second, title: 'Saved second', updated_at: 900 }
    const items = [{ ...response }, { ...first }]
    let acknowledged = [first, second]
    const { gateway } = createHomepageMediaSaveGateway(
      { second: response },
      { reorderFailure: true },
    )
    await expect(
      saveHomepageMediaChanges(items, acknowledged, {
        gateway,
        onProgress: (saved) => {
          acknowledged = saved
        },
      }),
    ).rejects.toThrow('reorder failed')
    expect(acknowledged).toEqual([first, response])
    expect(acknowledged[1]).not.toBe(items[0])
    const retry = createHomepageMediaSaveGateway({})
    await saveHomepageMediaChanges(items, acknowledged, { gateway: retry.gateway })
    expect(retry.calls).toEqual([{ method: 'reorder', ids: ['second', 'first'] }])
  })

  it('sends no requests and returns an independent snapshot when nothing changed', async () => {
    const items = [media(), media({ id: 'media-2', title: 'Forest', sort_order: 1 })]
    const savedItems = items.map((item) => ({ ...item }))
    const { calls, gateway } = createHomepageMediaSaveGateway({})

    const snapshot = await saveHomepageMediaChanges(items, savedItems, { gateway })

    expect(calls).toEqual([])
    expect(snapshot).toEqual(items)
    expect(snapshot).not.toBe(items)
    expect(snapshot[0]).not.toBe(items[0])
    expect(snapshot[1]).not.toBe(items[1])
  })

  it('patches only changed entries with exact image and panorama bodies', async () => {
    const savedImage = media({ id: 'image', title: 'Image before' })
    const savedStable = media({ id: 'stable', title: 'Stable', sort_order: 1 })
    const savedPanorama = media({
      id: 'panorama',
      type: 'panorama',
      title: 'Panorama before',
      sort_order: 2,
    })
    const image = { ...savedImage, title: 'Image after', enabled: false, duration_ms: 9000 }
    const stable = { ...savedStable }
    const panorama = {
      ...savedPanorama,
      title: 'Panorama after',
      overlay_opacity_light: 0.3,
      overlay_opacity_dark: 0.7,
      start_yaw: 45,
      start_pitch: -15,
      yaw_speed_dps: 3,
      pitch_speed_dps: -2,
    }
    const imageResponse = { ...image, updated_at: 301 }
    const panoramaResponse = {
      ...panorama,
      duration_ms: '10000' as unknown as number,
      updated_at: 302,
    }
    const { calls, gateway } = createHomepageMediaSaveGateway({
      image: imageResponse,
      panorama: panoramaResponse,
    })

    const snapshot = await saveHomepageMediaChanges(
      [image, stable, panorama],
      [savedImage, savedStable, savedPanorama],
      { gateway },
    )

    expect(calls).toEqual([
      {
        method: 'patch',
        id: 'image',
        body: {
          title: 'Image after',
          enabled: false,
          duration_ms: 9000,
          overlay_opacity_light: 0.2,
          overlay_opacity_dark: 0.4,
        },
      },
      {
        method: 'patch',
        id: 'panorama',
        body: {
          title: 'Panorama after',
          enabled: true,
          duration_ms: 8000,
          overlay_opacity_light: 0.3,
          overlay_opacity_dark: 0.7,
          start_yaw: 45,
          start_pitch: -15,
          yaw_speed_dps: 3,
          pitch_speed_dps: -2,
        },
      },
    ])
    expect(image).toEqual(imageResponse)
    expect(stable).toEqual(savedStable)
    expect(panorama).toEqual({ ...panoramaResponse, duration_ms: 10000 })
    expect(snapshot).toEqual([
      imageResponse,
      savedStable,
      { ...panoramaResponse, duration_ms: 10000 },
    ])
    expect(snapshot[0]).not.toBe(image)
    expect(snapshot[2]).not.toBe(panorama)
  })

  it('sends only the complete order when entries are reordered without field changes', async () => {
    const first = media({ id: 'first', title: 'First', sort_order: 0 })
    const second = media({ id: 'second', title: 'Second', sort_order: 1 })
    const third = media({ id: 'third', title: 'Third', sort_order: 2 })
    const items = [{ ...third }, { ...first }, { ...second }]
    const { calls, gateway } = createHomepageMediaSaveGateway({})

    const snapshot = await saveHomepageMediaChanges(items, [first, second, third], { gateway })

    expect(calls).toEqual([{ method: 'reorder', ids: ['third', 'first', 'second'] }])
    expect(snapshot).toEqual(items)
  })

  it('patches changed entries before sending the complete reordered id list', async () => {
    const savedFirst = media({ id: 'first', title: 'First', sort_order: 0 })
    const savedSecond = media({ id: 'second', title: 'Second', sort_order: 1 })
    const changedSecond = { ...savedSecond, title: 'Second updated', duration_ms: 12000 }
    const response = { ...changedSecond, updated_at: 500 }
    const items = [changedSecond, { ...savedFirst }]
    const { calls, gateway } = createHomepageMediaSaveGateway({ second: response })

    const snapshot = await saveHomepageMediaChanges(items, [savedFirst, savedSecond], { gateway })

    expect(calls).toEqual([
      {
        method: 'patch',
        id: 'second',
        body: {
          title: 'Second updated',
          enabled: true,
          duration_ms: 12000,
          overlay_opacity_light: 0.2,
          overlay_opacity_dark: 0.4,
        },
      },
      { method: 'reorder', ids: ['second', 'first'] },
    ])
    expect(snapshot).toEqual([response, savedFirst])
  })

  it('stops before later patches and reorder when a patch request fails', async () => {
    const savedFirst = media({ id: 'first', title: 'First', sort_order: 0 })
    const savedSecond = media({ id: 'second', title: 'Second', sort_order: 1 })
    const changedSecond = { ...savedSecond, title: 'Second changed' }
    const changedFirst = { ...savedFirst, title: 'First changed' }
    const { calls, gateway } = createHomepageMediaSaveGateway(
      { second: { ...changedSecond, updated_at: 300 } },
      { patchFailureId: 'second' },
    )

    await expect(
      saveHomepageMediaChanges([changedSecond, changedFirst], [savedFirst, savedSecond], {
        gateway,
      }),
    ).rejects.toThrow('patch failed: second')
    expect(calls).toEqual([
      {
        method: 'patch',
        id: 'second',
        body: {
          title: 'Second changed',
          enabled: true,
          duration_ms: 8000,
          overlay_opacity_light: 0.2,
          overlay_opacity_dark: 0.4,
        },
      },
    ])
    expect(changedSecond).toEqual({ ...savedSecond, title: 'Second changed' })
    expect(changedFirst).toEqual({ ...savedFirst, title: 'First changed' })
  })

  it('reports reorder failure after exact patches without producing a saved snapshot', async () => {
    const savedFirst = media({ id: 'first', title: 'First', sort_order: 0 })
    const savedSecond = media({ id: 'second', title: 'Second', sort_order: 1 })
    const changedSecond = { ...savedSecond, title: 'Second changed' }
    const response = { ...changedSecond, updated_at: 700 }
    const items = [changedSecond, { ...savedFirst }]
    const { calls, gateway } = createHomepageMediaSaveGateway(
      { second: response },
      { reorderFailure: true },
    )

    await expect(
      saveHomepageMediaChanges(items, [savedFirst, savedSecond], { gateway }),
    ).rejects.toThrow('reorder failed')
    expect(calls).toEqual([
      {
        method: 'patch',
        id: 'second',
        body: {
          title: 'Second changed',
          enabled: true,
          duration_ms: 8000,
          overlay_opacity_light: 0.2,
          overlay_opacity_dark: 0.4,
        },
      },
      { method: 'reorder', ids: ['second', 'first'] },
    ])
    expect(items).toEqual([response, savedFirst])
  })
})
