import { computed, reactive } from 'vue'
import { describe, expect, it } from 'vitest'
import { useDirtySnapshot } from '../useDirtySnapshot'

describe('dirty fields and groups share one confirmed baseline', () => {
  it('ignores uncaptured state and clears a field when restored to its original value', () => {
    const value = reactive({ name: 'before', enabled: false })
    const tracker = useDirtySnapshot(computed(() => value))
    value.name = 'loaded'
    expect(tracker.hasChanges.value).toBe(false)
    expect(tracker.hasFieldChanges('name')).toBe(false)
    tracker.capture()
    value.name = 'draft'
    value.enabled = true
    expect(tracker.hasFieldChanges('name')).toBe(true)
    expect(tracker.hasFieldChanges('enabled')).toBe(true)
    value.name = 'loaded'
    expect(tracker.hasFieldChanges('name')).toBe(false)
    expect(tracker.hasChanges.value).toBe(true)
    value.enabled = false
    expect(tracker.hasChanges.value).toBe(false)
  })

  it('captures only the selected group and reset removes its field baseline', () => {
    const value = reactive({ site: { name: 'site' }, auth: { days: 7 } })
    const site = useDirtySnapshot(computed(() => value.site))
    const auth = useDirtySnapshot(computed(() => value.auth))
    site.capture()
    auth.capture()
    value.site.name = 'new'
    value.auth.days = 14
    site.capture()
    expect(site.hasFieldChanges('name')).toBe(false)
    expect(auth.hasFieldChanges('days')).toBe(true)
    expect(auth.hasChanges.value).toBe(true)
    auth.reset()
    expect(auth.hasFieldChanges('days')).toBe(false)
    expect(auth.hasChanges.value).toBe(false)
  })
})
