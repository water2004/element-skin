import { defineStore } from 'pinia'
import { getMe } from '@/api/me'
import type { User } from '@/api/types'

interface SessionState {
  user: User | null
  authReady: boolean
}

export const useSessionStore = defineStore('session', {
  state: (): SessionState => ({
    user: null,
    authReady: false,
  }),
  getters: {
    isLogged: (state) => state.user !== null,
  },
  actions: {
    async fetchMe() {
      try {
        const res = await getMe()
        this.user = res.data
      } catch {
        this.user = null
      } finally {
        this.authReady = true
      }
    },
    clearSession() {
      this.user = null
    },
  },
})
