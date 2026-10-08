import { ref } from 'vue'
import { getNotices } from '@/api/notices'

const hasUnreadNotifications = ref(false)
const loadingUnreadNotifications = ref(false)

let refreshPromise: Promise<void> | null = null
let refreshGeneration = 0

async function refreshUnreadNotifications() {
  if (refreshPromise) return refreshPromise

  const generation = refreshGeneration
  loadingUnreadNotifications.value = true
  refreshPromise = getNotices({
    limit: 1,
    include_read: false,
  })
    .then((res) => {
      if (generation !== refreshGeneration) return
      hasUnreadNotifications.value =
        res.data.page_size > 0 || res.data.has_next || res.data.items.length > 0
    })
    .catch(() => {
      if (generation === refreshGeneration) hasUnreadNotifications.value = false
    })
    .finally(() => {
      if (generation === refreshGeneration) {
        loadingUnreadNotifications.value = false
        refreshPromise = null
      }
    })

  return refreshPromise
}

function clearUnreadNotifications() {
  refreshGeneration++
  hasUnreadNotifications.value = false
  loadingUnreadNotifications.value = false
  refreshPromise = null
}

export function useNotificationIndicator() {
  return {
    hasUnreadNotifications,
    loadingUnreadNotifications,
    refreshUnreadNotifications,
    clearUnreadNotifications,
  }
}
