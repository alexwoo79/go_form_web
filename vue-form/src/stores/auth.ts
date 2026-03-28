import { ref } from 'vue'
import { defineStore } from 'pinia'

export interface AuthUser {
  id: number
  username: string
  role: 'admin' | 'user'
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const checked = ref(false)

  async function fetchMe() {
    try {
      const res = await fetch('/api/me')
      if (res.ok) {
        user.value = await res.json()
      } else {
        user.value = null
      }
    } catch {
      user.value = null
    } finally {
      checked.value = true
    }
  }

  function setUser(u: AuthUser | null) {
    user.value = u
    checked.value = true
  }

  const isLoggedIn = () => user.value !== null
  const isAdmin = () => user.value?.role === 'admin'

  return { user, checked, fetchMe, setUser, isLoggedIn, isAdmin }
})
