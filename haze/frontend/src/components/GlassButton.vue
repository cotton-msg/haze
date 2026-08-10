<template>
  <button
    class="px-5 py-2.5 rounded-xl font-medium text-sm transition-all duration-200 cursor-pointer disabled:opacity-45 disabled:cursor-not-allowed select-none"
    :class="[
      variant === 'primary' ? 'text-white' : 'text-[var(--color-text)]',
      'active:scale-[0.97]',
    ]"
    :style="{
      background: variant === 'primary' ? 'linear-gradient(135deg, #6d5ee8, #8575f5)' : 'rgba(255,255,255,0.06)',
      border: variant === 'secondary' ? '1px solid rgba(255,255,255,0.1)' : 'none',
      boxShadow: hovered && variant === 'primary' ? '0 8px 24px -8px rgba(109,94,232,0.5)' : 'none',
      backdropFilter: variant === 'secondary' ? 'blur(12px)' : 'none',
    }"
    @mouseenter="hovered = true"
    @mouseleave="hovered = false"
  >
    <slot />
  </button>
</template>

<script setup lang="ts">
import { ref } from "vue"

withDefaults(defineProps<{ variant?: "primary" | "secondary" }>(), {
  variant: "primary",
})
const hovered = ref(false)
</script>
