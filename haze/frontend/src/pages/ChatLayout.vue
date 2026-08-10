<template>
  <div
    class="h-full w-full flex relative overflow-hidden"
    :style="{
      background:
        'radial-gradient(900px 600px at 8% 0%, rgba(124,108,240,0.10), transparent 60%), radial-gradient(800px 700px at 100% 100%, rgba(45,212,191,0.06), transparent 55%), #0c0e14',
    }"
  >
    <div
      class="glass flex w-80 shrink-0 h-full"
      :style="{ borderRadius: '0', borderRight: '1px solid rgba(255,255,255,0.08)', background: 'rgba(16,19,27,0.55)' }"
    >
      <Sidebar class="w-full h-full" />
    </div>
    <div class="flex-1 h-full flex flex-col min-w-0">
      <ChatWindow class="flex-1 h-full" />
    </div>
    <ProfileModal :show="profileOpen" @close="profileOpen = false" />
    <CallModal />
  </div>
</template>

<script setup lang="ts">
import { ref, provide, onMounted } from "vue"
import { useUserStore } from "../stores/userStore"
import { wsClient } from "../services/ws"
import { initPushNotifications } from "../services/push"
import Sidebar from "../components/Sidebar.vue"
import ChatWindow from "../components/ChatWindow.vue"
import ProfileModal from "../components/ProfileModal.vue"
import CallModal from "../components/CallModal.vue"

const store = useUserStore()
const profileOpen = ref(false)
provide("openProfile", () => {
  profileOpen.value = true
})

onMounted(async () => {
  if (!store.user) await store.fetchMe()
  wsClient.connect()
  initPushNotifications()
})
</script>
