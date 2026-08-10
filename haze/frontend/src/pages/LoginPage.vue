<template>
  <div
    class="h-full w-full flex items-center justify-center relative overflow-hidden"
    :style="{
      background:
        'radial-gradient(700px 500px at 20% 10%, rgba(124,108,240,0.16), transparent 60%), radial-gradient(700px 500px at 85% 90%, rgba(45,212,191,0.10), transparent 60%), #0c0e14',
    }"
  >
    <div
      class="glass haze-fade-up rounded-3xl p-10 w-full max-w-sm mx-4 text-center"
      :style="{ background: 'linear-gradient(165deg, rgba(30,34,46,0.72), rgba(18,21,29,0.66))' }"
    >
      <div class="mb-7">
        <div
          class="mx-auto mb-4 w-14 h-14 rounded-2xl flex items-center justify-center text-2xl font-bold"
          :style="{
            background: 'linear-gradient(135deg, #6d5ee8, #2dd4bf)',
            color: '#fff',
            boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.25), 0 10px 30px -10px rgba(109,94,232,0.6)',
          }"
        >
          H
        </div>
        <h1 class="text-2xl font-semibold tracking-tight">Haze</h1>
        <p class="text-sm mt-1.5" :style="{ color: 'var(--color-text-muted)' }">тихий мессенджер</p>
      </div>

      <GlassButton class="w-full" @click="loginWithSSA">
        Войти через SSA
      </GlassButton>
      <p class="text-xs mt-5" :style="{ color: 'var(--color-text-muted)' }">
        Продолжая, вы соглашаетесь с условиями сервиса
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import GlassButton from "../components/GlassButton.vue"

function loginWithSSA() {
  window.location.href = "/api/auth/ssa/authorize"
}
</script>
