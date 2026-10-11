import { beforeEach, describe, expect, it, vi } from 'vitest'
import { appStorage } from '../storage'
import { normalizePageSize } from '../pageSize'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { pageSizeCases } from './fixtures/pageSizeCases'

beforeEach(() => window.localStorage.clear())

describe('page size preferences', () => {
  it.each(pageSizeCases)(
    'reads $label preferences with an exact page size',
    ({ stored, expected }) => {
      if (stored !== null) window.localStorage.setItem('page_size_preference', stored)
      expect(appStorage.pageSize.get()).toBe(expected)
      expect(useCursorPagination().limit.value).toBe(expected)
    },
  )

  it('uses the requested default when missing or when storage reads fail', () => {
    expect(appStorage.pageSize.get(30)).toBe(30)
    expect(useCursorPagination(30).limit.value).toBe(30)
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage unavailable')
    })
    expect(appStorage.pageSize.get(30)).toBe(30)
    expect(useCursorPagination(30).limit.value).toBe(30)
  })

  it('normalizes invalid types and fallback values without interpreting them as zero', () => {
    expect(normalizePageSize(null, 30)).toBe(30)
    expect(normalizePageSize(false, 30)).toBe(30)
    expect(normalizePageSize([], 30)).toBe(30)
    expect(normalizePageSize('', Number.NaN)).toBe(20)
    expect(normalizePageSize(undefined, 500)).toBe(100)
  })

  it('resets navigation and persists a changed size for subsequently opened lists', async () => {
    const page = useCursorPagination<string>()
    page.setPageData({ items: ['first'], has_next: true, next_cursor: 'next', page_size: 20 })
    const fetchPage = vi.fn().mockResolvedValue({
      items: ['second'],
      has_next: true,
      next_cursor: 'third',
      page_size: 20,
    })
    await page.goToNextPage(fetchPage)
    expect(fetchPage).toHaveBeenCalledExactlyOnceWith('next', 20)
    expect(page.currentCursor.value).toBe('next')
    expect(page.hasPrev.value).toBe(true)
    expect(page.setLimit('10.6')).toBe(true)
    expect(page.items.value).toEqual([])
    expect(page.currentCursor.value).toBeNull()
    expect(page.nextCursor.value).toBeNull()
    expect(page.hasNext.value).toBe(false)
    expect(page.hasPrev.value).toBe(false)
    expect(page.limit.value).toBe(11)
    expect(window.localStorage.getItem('page_size_preference')).toBe('11')
    expect(useCursorPagination().limit.value).toBe(11)
    const fetchPrevious = vi.fn()
    await page.goToPrevPage(fetchPrevious)
    expect(fetchPrevious).not.toHaveBeenCalled()
  })

  it('preserves the page on invalid or unchanged input', () => {
    const page = useCursorPagination<string>()
    const response = { items: ['kept'], has_next: true, next_cursor: 'next', page_size: 20 }
    page.setPageData(response)
    expect(page.setLimit('')).toBe(false)
    expect(page.setLimit('invalid')).toBe(false)
    expect(page.setLimit(20)).toBe(false)
    expect(page.items.value).toEqual(['kept'])
    expect(page.nextCursor.value).toBe('next')
    expect(window.localStorage.getItem('page_size_preference')).toBeNull()
  })

  it('keeps an in-memory size change usable when persistence exceeds quota', () => {
    window.localStorage.setItem('page_size_preference', '20')
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('quota exceeded', 'QuotaExceededError')
    })
    const page = useCursorPagination<string>()
    page.setPageData({ items: ['old'], has_next: true, next_cursor: 'next', page_size: 20 })
    expect(page.setLimit(30)).toBe(true)
    expect(page.limit.value).toBe(30)
    expect(page.currentCursor.value).toBeNull()
    expect(page.items.value).toEqual([])
    expect(window.localStorage.getItem('page_size_preference')).toBe('20')
  })
})
