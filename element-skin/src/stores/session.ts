import { defineStore } from 'pinia'
import { getMe } from '@/api/me'
import type { User } from '@/api/types'
import { useAvatar } from '@/composables/useAvatar'
import { useNotificationIndicator } from '@/composables/useNotificationIndicator'

interface SessionState {
  user: User | null
  authReady: boolean
  generation: number
}

export const useSessionStore = defineStore('session', {
  state: (): SessionState => ({
    user: null,
    authReady: false,
    generation: 0,
  }),
  getters: {
    isLogged: (state) => state.user !== null,
  },
  actions: {
    async fetchMe() {
      const generation = this.generation
      try {
        const res = await getMe()
        if (generation === this.generation) this.user = res.data
      } catch {
        if (generation === this.generation) this.clearSession()
      } finally {
        if (generation === this.generation) this.authReady = true
      }
    },
    clearSession() {
      this.generation++
      this.user = null
      this.authReady = true
      void useAvatar().initializeAvatar(null)
      useNotificationIndicator().clearUnreadNotifications()
    },
  },
})
