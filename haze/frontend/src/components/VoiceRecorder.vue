<template>
  <div class="flex items-center gap-3">
    <button
      v-if="!recording"
      class="w-10 h-10 rounded-full flex items-center justify-center text-lg transition-all hover:bg-[rgba(255,255,255,0.09)]"
      :style="{ background: 'rgba(255,255,255,0.05)', color: '#e8eaf0', border: '1px solid rgba(255,255,255,0.08)' }"
      @mousedown="startRecording"
      @mouseup="stopRecording"
      @mouseleave="cancelRecording"
    >
      🎤
    </button>

    <div v-else class="glass-soft flex items-center gap-3 px-3 py-2 rounded-xl">
      <div class="flex items-center gap-1">
        <span
          v-for="i in 20"
          :key="i"
          class="w-1 rounded-full transition-all"
          :style="{ height: getBarHeight(i), background: '#f4586c' }"
        />
      </div>
      <span class="text-sm font-mono" :style="{ color: '#f4586c' }">{{ formattedTime }}</span>
      <button
        class="w-8 h-8 rounded-full flex items-center justify-center text-xs"
        :style="{ background: '#f4586c', color: '#fff' }"
        @click="cancelRecording"
      >✕</button>
      <button
        class="w-8 h-8 rounded-full flex items-center justify-center text-xs"
        :style="{ background: '#2dd4bf', color: '#0c0e14' }"
        @click="sendRecording"
      >✓</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue"

const emit = defineEmits<{ send: [blob: Blob, duration: number, waveform: number[]]; cancel: [] }>()

const recording = ref(false)
const elapsed = ref(0)
const mediaRecorder = ref<MediaRecorder | null>(null)
const chunks = ref<Blob[]>([])
let timer: ReturnType<typeof setInterval> | null = null
let stream: MediaStream | null = null
let audioCtx: AudioContext | null = null
let analyser: AnalyserNode | null = null
let samples: number[] = []

const formattedTime = computed(() => {
  const s = elapsed.value
  return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`
})

function getBarHeight(i: number): string {
  const h = Math.sin((Date.now() / 200 + i) * 0.5) * 10 + 12
  return `${h}px`
}

async function startRecording() {
  try {
    stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    audioCtx = new AudioContext()
    analyser = audioCtx.createAnalyser()
    analyser.fftSize = 128
    const source = audioCtx.createMediaStreamSource(stream)
    source.connect(analyser)
    samples = []

    const mr = new MediaRecorder(stream)
    mediaRecorder.value = mr
    chunks.value = []

    mr.ondataavailable = (e) => {
      if (e.data.size > 0) chunks.value.push(e.data)
    }

    mr.onstop = () => {
      stream?.getTracks().forEach((t) => t.stop())
      audioCtx?.close()
      audioCtx = null
      analyser = null
    }

    mr.start(100)
    recording.value = true
    elapsed.value = 0

    timer = setInterval(() => {
      elapsed.value++
      if (analyser) {
        const data = new Uint8Array(analyser.frequencyBinCount)
        analyser.getByteFrequencyData(data)
        let sum = 0
        for (const v of data) sum += v
        samples.push(Math.round((sum / data.length / 255) * 100))
      }
      if (elapsed.value >= 60) stopRecording()
    }, 1000)
  } catch {
    recording.value = false
  }
}

function stopRecording() {
  if (timer) clearInterval(timer)
  if (mediaRecorder.value && mediaRecorder.value.state !== "inactive") {
    mediaRecorder.value.stop()
  }
}

function cancelRecording() {
  stopRecording()
  if (stream) stream.getTracks().forEach((t) => t.stop())
  recording.value = false
  elapsed.value = 0
  emit("cancel")
}

function sendRecording() {
  stopRecording()
  const blob = new Blob(chunks.value, { type: "audio/webm" })
  recording.value = false
  elapsed.value = 0
  emit("send", blob, Math.max(1, Math.round(elapsed.value)), buildWaveform(samples))
}

function buildWaveform(samples: number[]): number[] {
  const bars = 40
  const out: number[] = []
  if (!samples.length) {
    for (let i = 0; i < bars; i++) out.push(Math.round(Math.sin((i / bars) * Math.PI) * 60 + 15))
    return out
  }
  for (let i = 0; i < bars; i++) {
    const start = Math.floor((i / bars) * samples.length)
    const end = Math.max(start + 1, Math.floor(((i + 1) / bars) * samples.length))
    let sum = 0
    for (let j = start; j < end; j++) sum += samples[j]
    out.push(Math.round(sum / (end - start)))
  }
  return out
}
</script>
