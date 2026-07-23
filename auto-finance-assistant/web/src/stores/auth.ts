import { defineStore } from 'pinia'
import { ref } from 'vue'

const STORAGE_KEY = 'admin_password'

export const useAuthStore = defineStore('auth', () => {
  const password = ref<string>(localStorage.getItem(STORAGE_KEY) || '')

  function setPassword(pw: string) {
    password.value = pw
    if (pw) localStorage.setItem(STORAGE_KEY, pw)
    else localStorage.removeItem(STORAGE_KEY)
  }

  function clear() {
    password.value = ''
    localStorage.removeItem(STORAGE_KEY)
  }

  // authedFetch: 对需认证的接口自动附加 X-Admin-Password 头
  async function authedFetch(url: string, options: RequestInit = {}): Promise<Response> {
    const headers = new Headers(options.headers)
    if (password.value) {
      headers.set('X-Admin-Password', password.value)
    }
    return fetch(url, { ...options, headers })
  }

  return { password, setPassword, clear, authedFetch }
})
