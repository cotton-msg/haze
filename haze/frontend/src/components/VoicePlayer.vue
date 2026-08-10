<template>
  <div
    class="glass-soft flex items-center gap-3 px-4 py-2 rounded-xl w-64"
  >
    <button
      class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 transition-transform active:scale-90"
      :style="{ background: 'rgba(124,108,240,0.22)', color: '#cdc6ff' }"
      @click="togglePlay"
    >
      {{ playing ? "⏸" : "▶️" }}
    </button>

    <div class="flex-1 h-8 flex items-center gap-0.5">
      <div
        v-for="(h, i) in waveform"
        :key="i"
        class="w-1 rounded-full transition-all duration-100"
        :style="{
          height: h + 'px',
          background: i / waveform.length < progress ? '#6d5ee8' : 'rgba(255,255,255,0.16)',
        }"
      />
    </div>

    <span class="text-xs font-mono shrink-0" :style="{ color: 'var(--color-text-muted)' }">
      {{ displayTime }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted } from "vue"

const props = defineProps<{ src: string; duration?: number; waveform?: number[] }>()

const playing = ref(false)
const progress = ref(0)
const currentTime = ref(0)

const audio = ref<HTMLAudioElement | null>(null)

const waveform = computed(() => {
  const bars = 40
  const real = props.waveform || []
  return Array.from({ length: bars }, (_, i) => {
    if (real.length) {
      const idx = Math.floor((i / bars) * real.length)
      return Math.max(4, Math.round((real[Math.min(idx, real.length - 1)] / 100) * 30))
    }
    const base = Math.sin((i / bars) * Math.PI) * 18 + 6
    if (playing.value) {
      return base + Math.sin((Date.now() / 200 + i) * 0.5) * 4
    }
    return base
  })
})

const displayTime = computed(() => {
  const t = playing.value ? currentTime.value : (props.duration || 0)
  const s = Math.floor(t)
  return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`
})

function togglePlay() {
  if (!audio.value) {
    audio.value = new Audio(props.src)
    audio.value.onended = () => { playing.value = false; progress.value = 0 }
    audio.value.ontimeupdate = () => {
      if (audio.value && audio.value.duration) {
        progress.value = audio.value.currentTime / audio.value.duration
        currentTime.value = audio.value.currentTime
      }
    }
  }

  if (playing.value) {
    audio.value.pause()
  } else {
    audio.value.play()
  }
  playing.value = !playing.value
}

onUnmounted(() => {
  audio.value?.pause()
  audio.value = null
})
</script>
