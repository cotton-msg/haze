<template>
  <div
    class="glass-soft rounded-xl p-3 flex items-center gap-3 cursor-pointer transition-all hover:bg-[rgba(255,255,255,0.08)]"
    @click="download"
  >
    <div
      class="w-10 h-10 rounded-lg flex items-center justify-center text-lg shrink-0"
      :style="{ background: 'rgba(124,108,240,0.18)', color: '#cdc6ff' }"
    >
      {{ icon }}
    </div>
    <div class="flex-1 min-w-0">
      <p class="text-sm font-medium truncate">{{ name }}</p>
      <p class="text-xs mt-0.5" :style="{ color: 'var(--color-text-muted)' }">{{ size }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{ name: string; size: number; url: string }>()

const icon = computed(() => {
  const ext = props.name.split(".").pop()?.toLowerCase()
  if (["png", "jpg", "jpeg", "gif", "webp"].includes(ext || "")) return "🖼"
  if (["mp4", "webm", "mov"].includes(ext || "")) return "🎬"
  if (["mp3", "wav", "ogg"].includes(ext || "")) return "🎵"
  if (["pdf"].includes(ext || "")) return "📄"
  if (["zip", "rar", "7z"].includes(ext || "")) return "📦"
  return "📎"
})

const size = computed(() => {
  const bytes = props.size
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
})

function download() {
  window.open(props.url, "_blank")
}
</script>
