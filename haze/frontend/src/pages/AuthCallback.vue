<template>
  <div class="h-full w-full flex items-center justify-center">
    <div class="flex items-center gap-3 text-sm" :style="{ color: 'var(--color-text-muted)' }">
      <div class="glass-soft rounded-xl px-4 py-2 flex items-center gap-2">
        <span class="w-3.5 h-3.5 rounded-full border-2 border-transparent" :style="{ borderTopColor: '#6d5ee8', animation: 'haze-spin 0.8s linear infinite' }" />
        Авторизация…
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import { useRouter, useRoute } from "vue-router"
import { useUserStore } from "../stores/userStore"

const router = useRouter()
const route = useRoute()
const store = useUserStore()

onMounted(async () => {
  const code = route.query.code as string
  const state = route.query.state as string
  if (!code || !state) {
    router.push("/login")
    return
  }

  try {
    const res: any = await fetch("/api/auth/ssa/callback?" + new URLSearchParams({ code, state }), {
      method: "POST",
    }).then((r) => r.json())

    if (!res.error && res.data) {
      store.setTokens(res.data.tokens.access_token, res.data.tokens.refresh_token)
      store.user = res.data.user
      router.push("/chats")
    } else {
      router.push("/login")
    }
  } catch {
    router.push("/login")
  }
})
</script>

<style>
@keyframes haze-spin {
  to { transform: rotate(360deg); }
}
</style>
