import { defineStore } from "pinia"
import { ref } from "vue"
import { api } from "../services/api"

export interface User {
  id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
  bio: string
  role: string
  is_premium: boolean
}

export const useUserStore = defineStore("user", () => {
  const user = ref<User | null>(null)
  const loading = ref(false)

  async function fetchMe() {
    loading.value = true
    try {
      const res: any = await api.get("/auth/me")
      if (!res.error) user.value = res.data
    } catch {
      user.value = null
    } finally {
      loading.value = false
    }
  }

  async function updateProfile(patch: Partial<User>) {
    try {
      const res: any = await api.put("/auth/me", patch)
      if (!res.error && res.data) user.value = res.data
      return !res.error
    } catch {
      return false
    }
  }

  // Загрузка аватара в media и обновление профиля.
  async function uploadAvatar(file: File): Promise<boolean> {
    try {
      const token = localStorage.getItem("access_token")
      const form = new FormData()
      form.append("file", file)
      const res = await fetch("/api/media/upload", {
        method: "POST",
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body: form,
      })
      const data = await res.json()
      if (data.error || !data.data?.url) return false
      return updateProfile({ avatar_url: data.data.url })
    } catch {
      return false
    }
  }

  function setTokens(access: string, refresh: string) {
    localStorage.setItem("access_token", access)
    localStorage.setItem("refresh_token", refresh)
  }

  function logout() {
    user.value = null
    localStorage.clear()
  }

  return { user, loading, fetchMe, updateProfile, uploadAvatar, setTokens, logout }
})
