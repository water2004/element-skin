import { computed, ref, type Ref } from 'vue'

/**
 * Track whether a reactive source has unsaved changes by comparing JSON snapshots.
 *
 * Captures a baseline snapshot on `capture()` (typically after load or save),
 * and exposes a reactive `hasChanges` flag that flips true when the current
 * state diverges from the baseline.
 *
 * @param source a ref or computed that returns the value to track
 */
export function useDirtySnapshot<T>(source: Ref<T>) {
  const snapshot = ref('')
  const baseline = computed<T | undefined>(() =>
    snapshot.value === '' ? undefined : JSON.parse(snapshot.value),
  )

  const hasChanges = computed(
    () => snapshot.value !== '' && snapshot.value !== JSON.stringify(source.value),
  )

  function capture() {
    snapshot.value = JSON.stringify(source.value)
  }

  function reset() {
    snapshot.value = ''
  }

  function hasFieldChanges<K extends keyof T>(field: K): boolean {
    return (
      snapshot.value !== '' &&
      JSON.stringify(source.value?.[field]) !== JSON.stringify(baseline.value?.[field])
    )
  }

  return { hasChanges, hasFieldChanges, capture, reset }
}
