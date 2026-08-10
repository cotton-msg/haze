<template>
  <div class="h-full flex flex-col">
    <div class="p-4 pb-2 flex items-center gap-3">
      <button
        class="cursor-pointer transition-transform hover:scale-105"
        @click="openProfile"
        title="Профиль"
      >
        <Avatar :src="store.user?.avatar_url" :alt="store.user?.display_name || 'Пользователь'" :size="38" />
      </button>
      <GlassInput v-model="search" placeholder="Поиск" class="glass-soft flex-1" />
    </div>

    <div class="flex-1 overflow-y-auto p-2 space-y-0.5">
      <div
        v-for="chat in filteredChats"
        :key="chat.id"
        class="glass-soft flex items-center gap-3 p-2.5 rounded-xl cursor-pointer transition-all duration-200"
        :class="chat.id === activeChat ? '' : 'hover:bg-[rgba(255,255,255,0.07)]'"
        :style="{
          background: chat.id === activeChat
            ? 'linear-gradient(135deg, rgba(124,108,240,0.22), rgba(124,108,240,0.10))'
            : 'rgba(255,255,255,0.03)',
          border: chat.id === activeChat ? '1px solid rgba(124,108,240,0.35)' : '1px solid transparent',
          backdropFilter: 'blur(10px)',
        }"
        @click="selectChat(chat)"
      >
        <Avatar :alt="chat.title" :size="42" />
        <div class="flex-1 min-w-0">
          <div class="flex justify-between items-center">
            <span class="text-sm font-medium truncate">{{ chat.title }}</span>
            <span class="text-xs shrink-0 ml-2" :style="{ color: 'var(--color-text-muted)' }">
              {{ timeAgo(chat.updated_at) }}
            </span>
          </div>
          <div class="flex justify-between items-center mt-0.5 gap-2">
            <p
              v-if="chat.last_message"
              class="text-xs truncate"
              :style="{ color: 'var(--color-text-muted)' }"
            >
              {{ chat.last_message }}
            </p>
            <span
              v-if="(chat.unread_count || 0) > 0"
              class="text-[10px] shrink-0 px-1.5 py-0.5 rounded-full font-medium"
              :style="{ background: 'linear-gradient(135deg, #6d5ee8, #8575f5)', color: '#fff' }"
            >
              {{ chat.unread_count }}
            </span>
          </div>
        </div>
      </div>

      <p
        v-if="chatStore.chats.length === 0"
        class="text-center text-sm pt-16"
        :style="{ color: 'var(--color-text-muted)' }"
      >
        Пока нет диалогов
      </p>
    </div>

    <div class="p-3 border-t" :style="{ borderColor: 'rgba(255,255,255,0.07)' }">
      <button
        class="w-full text-left text-xs cursor-pointer hover:opacity-80 transition-opacity"
        :style="{ color: 'var(--color-text-muted)' }"
        @click="logout"
      >
        Выйти
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, inject } from "vue"
import { useRouter, useRoute } from "vue-router"
import { useChatStore, type Chat } from "../stores/chatStore"
import { useUserStore } from "../stores/userStore"
import GlassInput from "./GlassInput.vue"
import Avatar from "./Avatar.vue"

const search = ref("")
const chatStore = useChatStore()
const store = useUserStore()
const router = useRouter()
const route = useRoute()
const openProfile = inject<() => void>("openProfile", () => {})

const activeChat = ref(route.params.id as string)

onMounted(() => chatStore.fetchChats())

watch(
  () => route.params.id,
  (id) => {
    if (id) activeChat.value = id as string
  },
)

const filteredChats = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return chatStore.chats
  return chatStore.chats.filter((c) => c.title.toLowerCase().includes(q))
})

function selectChat(chat: Chat) {
  activeChat.value = chat.id
  chatStore.setCurrentChat(chat)
  router.push(`/chats/${chat.id}`)
}

function logout() {
  store.logout()
  router.push("/login")
}

function timeAgo(date: string): string {
  const diff = Date.now() - new Date(date).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}м`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}ч`
  return `${Math.floor(hours / 24)}д`
}
</script>
