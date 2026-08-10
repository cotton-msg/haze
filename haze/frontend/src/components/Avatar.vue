<template>
  <div class="relative inline-flex shrink-0" :style="{ width: size + 'px', height: size + 'px' }">
    <img
      v-if="src"
      :src="src"
      :alt="alt"
      class="rounded-full object-cover w-full h-full"
      :style="{ border: '1px solid rgba(255,255,255,0.14)' }"
    />
    <div
      v-else
      class="rounded-full w-full h-full flex items-center justify-center text-sm font-semibold"
      :style="{
        background: 'linear-gradient(135deg, rgba(124,108,240,0.9), rgba(45,212,191,0.85))',
        color: '#fff',
        boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.25)',
      }"
    >
      {{ initials }}
    </div>
    <span
      v-if="online"
      class="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full border-2"
      :style="{ background: '#2dd4bf', borderColor: '#0c0e14' }"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = withDefaults(
  defineProps<{ src?: string; alt?: string; size?: number; online?: boolean }>(),
  { size: 40, online: false },
)

const initials = computed(() => {
  if (!props.alt) return "?"
  return props.alt
    .split(" ")
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2)
})
</script>
