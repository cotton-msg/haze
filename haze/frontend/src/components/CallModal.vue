<template>
  <teleport to="body">
    <div
      v-if="callStore.call"
      class="fixed inset-0 z-50 flex items-center justify-center haze-fade-in"
      :style="{ background: 'rgba(8,10,15,0.85)', backdropFilter: 'blur(14px)', WebkitBackdropFilter: 'blur(14px)' }"
    >
      <div class="w-full max-w-md mx-4 haze-pop">
        <div class="glass rounded-3xl overflow-hidden" :style="{ background: 'linear-gradient(165deg, rgba(30,34,46,0.94), rgba(18,21,29,0.9))' }">
          <!-- Видео -->
          <div v-if="callStore.call?.type === 'video'" class="relative h-80 bg-black/50">
            <video
              ref="remoteVideo"
              autoplay playsinline
              class="w-full h-full object-cover"
            />
            <video
              ref="localVideo"
              autoplay playsinline muted
              class="absolute bottom-3 right-3 w-28 h-20 rounded-lg object-cover border border-white/10"
            />
            <div v-if="callStore.videoOff" class="absolute inset-0 flex items-center justify-center">
              <Avatar :alt="peerName" :size="72" />
            </div>
          </div>

          <!-- Аудио / шапка -->
          <div v-else class="pt-10 pb-2 flex flex-col items-center gap-3">
            <Avatar :alt="peerName" :size="88" />
          </div>

          <div class="text-center pt-4">
            <h2 class="text-xl font-semibold">{{ peerName }}</h2>
            <p class="text-sm mt-1" :style="{ color: 'var(--color-text-muted)' }">{{ statusText }}</p>
            <p v-if="callStore.call?.status === 'active'" class="text-xs mt-1 font-mono" :style="{ color: 'var(--color-text-muted)' }">
              {{ duration }}
            </p>
          </div>

          <!-- Кнопки -->
          <div class="flex items-center justify-center gap-4 p-8">
            <template v-if="callStore.direction === 'incoming' && callStore.call?.status === 'ringing'">
              <button class="call-btn" :style="{ background: '#2dd4bf', color: '#0c0e14' }" @click="callStore.answerCall()">
                <span class="text-xl">📞</span>
              </button>
              <button class="call-btn" :style="{ background: '#f4586c', color: '#fff' }" @click="callStore.rejectCall()">
                <span class="text-xl">✕</span>
              </button>
            </template>
            <template v-else-if="callStore.call?.status === 'ringing'">
              <button class="call-btn" :style="{ background: '#f4586c', color: '#fff' }" @click="callStore.endCall()">
                <span class="text-xl">✕</span>
              </button>
            </template>
            <template v-else>
              <button class="call-btn" :style="{ background: muted ? '#e8eaf0' : 'rgba(255,255,255,0.08)', color: muted ? '#0c0e14' : '#e8eaf0' }" @click="callStore.toggleMute()">
                <span class="text-xl">{{ muted ? "🔇" : "🎤" }}</span>
              </button>
              <button v-if="callStore.call?.type === 'video'" class="call-btn" :style="{ background: 'rgba(255,255,255,0.08)', color: '#e8eaf0' }" @click="callStore.toggleVideo()">
                <span class="text-xl">{{ videoOff ? "🎥✕" : "🎥" }}</span>
              </button>
              <button class="call-btn" :style="{ background: '#f4586c', color: '#fff' }" @click="callStore.endCall()">
                <span class="text-xl">✕</span>
              </button>
            </template>
          </div>
        </div>
      </div>
    </div>
  </teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from "vue"
import { useCallStore } from "../stores/callStore"
import { useUserStore } from "../stores/userStore"
import Avatar from "./Avatar.vue"

const callStore = useCallStore()
const userStore = useUserStore()

const remoteVideo = ref<HTMLVideoElement | null>(null)
const localVideo = ref<HTMLVideoElement | null>(null)
const elapsed = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const peerName = computed(() => {
  const me = userStore.user?.id
  if (!callStore.call) return ""
  return callStore.call.caller_id === me ? callStore.call.callee_id : callStore.call.caller_id
})

const statusText = computed(() => {
  if (callStore.call?.status === "ringing") {
    return callStore.direction === "incoming" ? "Входящий звонок…" : "Звоним…"
  }
  if (callStore.call?.status === "active") return "В разговоре"
  return "Завершён"
})

const duration = computed(() => {
  const s = elapsed.value
  return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`
})

const muted = computed(() => callStore.muted)
const videoOff = computed(() => callStore.videoOff)

watch(
  () => callStore.remoteStream,
  (stream) => {
    if (remoteVideo.value && stream) remoteVideo.value.srcObject = stream
  },
  { immediate: true },
)

watch(
  () => callStore.localStream,
  (stream) => {
    if (localVideo.value && stream) localVideo.value.srcObject = stream
  },
  { immediate: true },
)

watch(
  () => callStore.call?.status,
  (status) => {
    if (status === "active") {
      elapsed.value = 0
      timer = setInterval(() => elapsed.value++, 1000)
    } else if (timer) {
      clearInterval(timer)
      timer = null
    }
  },
)

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.call-btn {
  width: 60px;
  height: 60px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 8px 24px -10px rgba(0, 0, 0, 0.5);
  transition: transform 0.15s;
}
.call-btn:active {
  transform: scale(0.92);
}
</style>
