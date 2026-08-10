<template>
  <div class="flex-1 h-full flex flex-col min-w-0">
    <template v-if="chatStore.currentChat">
      <div
        class="h-14 px-5 flex items-center gap-3 shrink-0"
        :style="{
          background: 'rgba(16,19,27,0.35)',
          borderBottom: '1px solid rgba(255,255,255,0.07)',
          backdropFilter: 'blur(14px)',
        }"
      >
        <Avatar :alt="chatStore.currentChat.title" :size="32" />
        <div class="flex-1 min-w-0">
          <span class="font-medium text-sm truncate">{{ chatStore.currentChat.title }}</span>
          <p
            v-if="typingNames.length"
            class="text-xs truncate"
            :style="{ color: 'var(--color-accent, #cdc6ff)' }"
          >
            {{ typingNames.join(", ") }} печатает…
          </p>
        </div>
        <GlassButton variant="secondary" class="px-3 py-1.5 text-xs" @click="callVideo">Видеозвонок</GlassButton>
      </div>

      <div class="flex-1 overflow-y-auto p-6 space-y-2" ref="msgContainer" @scroll="onScroll">
        <div v-if="chatStore.loading" class="text-center text-xs pt-4" :style="{ color: 'var(--color-text-muted)' }">
          Загрузка…
        </div>
        <button
          v-if="chatStore.hasMore && !chatStore.loading"
          class="mx-auto block text-xs mb-2 cursor-pointer"
          :style="{ color: 'var(--color-text-muted)' }"
          @click="loadOlder"
        >
          Загрузить раньше
        </button>

        <div
          v-for="msg in chatStore.messagesChrono"
          :key="msg.id"
          class="flex haze-fade-up"
          :class="msg.sender_id === store.user?.id ? 'justify-end' : 'justify-start'"
        >
          <div
            class="max-w-[70%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed cursor-context-menu"
            :style="msg.sender_id === store.user?.id
              ? {
                  background: 'linear-gradient(135deg, rgba(109,94,232,0.92), rgba(133,117,245,0.88))',
                  color: '#fff',
                  boxShadow: '0 8px 24px -10px rgba(109,94,232,0.5)',
                }
              : {
                  background: 'rgba(255,255,255,0.055)',
                  color: '#e8eaf0',
                  border: '1px solid rgba(255,255,255,0.08)',
                  backdropFilter: 'blur(12px)',
                }"
            @contextmenu.prevent="openMenu(msg, $event)"
          >
            <div
              v-if="msg.reply_to"
              class="text-[11px] opacity-80 mb-1 pl-2 border-l-2 truncate"
              :style="{ borderColor: 'var(--color-accent, #cdc6ff)' }"
            >
              Ответ на: {{ replyPreview(msg.reply_to) }}
            </div>
            <img
              v-if="isImage(msg)"
              :src="msg.content"
              class="rounded-xl max-w-full max-h-72 object-contain cursor-pointer"
              @click="previewImage(msg.content)"
            />
            <FileMessage
              v-else-if="isFile(msg)"
              :name="attachmentName(msg)"
              :size="attachmentSize(msg)"
              :url="msg.content"
            />
            <VoicePlayer
              v-else-if="isVoice(msg)"
              :src="msg.content"
              :duration="attachmentDuration(msg)"
              :waveform="attachmentWaveform(msg)"
            />
            <p v-else>{{ msg.content }}</p>

            <a
              v-if="msg.link_preview"
              :href="msg.link_preview.url"
              target="_blank"
              rel="noopener noreferrer"
              class="mt-2 flex flex-col overflow-hidden rounded-xl border"
              :style="{ background: 'rgba(0,0,0,0.25)', borderColor: 'rgba(255,255,255,0.1)' }"
              @click.stop
            >
              <img
                v-if="msg.link_preview.image"
                :src="msg.link_preview.image"
                class="w-full max-h-40 object-cover"
                @error="(e) => ((e.target as HTMLImageElement).style.display = 'none')"
              />
              <div class="px-3 py-2">
                <p class="text-[13px] font-semibold leading-snug truncate">{{ msg.link_preview.title }}</p>
                <p v-if="msg.link_preview.description" class="text-xs opacity-80 line-clamp-2">
                  {{ msg.link_preview.description }}
                </p>
                <p class="text-[11px] opacity-60 mt-0.5 truncate">{{ hostname(msg.link_preview.url) }}</p>
              </div>
            </a>

            <div class="flex items-center gap-1 justify-end mt-1" v-if="msg.sender_id === store.user?.id">
              <span v-if="msg.edited_at" class="text-[10px] opacity-70">изменено</span>
              <span class="text-[10px] opacity-70">{{ time(msg.created_at) }}</span>
              <span class="text-[11px]" :style="{ color: statusColor(msg.status) }">
                {{ statusIcon(msg.status) }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div
        class="p-4"
        :style="{
          background: 'rgba(16,19,27,0.35)',
          borderTop: '1px solid rgba(255,255,255,0.07)',
          backdropFilter: 'blur(14px)',
        }"
      >
        <div v-if="attachments.length" class="flex flex-wrap gap-2 mb-2">
          <div
            v-for="(a, i) in attachments"
            :key="i"
            class="relative w-14 h-14 rounded-lg overflow-hidden glass-soft"
          >
            <img v-if="a.preview" :src="a.preview" class="w-full h-full object-cover" />
            <div v-else class="w-full h-full flex items-center justify-center text-xs">{{ a.name.slice(0, 4) }}</div>
            <button
              class="absolute top-0.5 right-0.5 w-5 h-5 rounded-full flex items-center justify-center text-xs"
              :style="{ background: 'rgba(0,0,0,0.55)' }"
              @click="removeAttachment(i)"
            >✕</button>
          </div>
        </div>

        <div class="relative">
          <div
            v-if="replyTo || editingMsg"
            class="flex items-center gap-2 mb-2 px-3 py-2 rounded-xl text-xs"
            :style="{ background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.08)' }"
          >
            <span v-if="editingMsg" class="truncate">Редактирование: {{ preview(editingMsg) }}</span>
            <span v-else-if="replyTo" class="truncate">Ответ на: {{ preview(replyTo) }}</span>
            <button class="ml-auto shrink-0 opacity-70 hover:opacity-100" @click="cancelAction">✕</button>
          </div>

          <form class="flex gap-2.5 items-center" @submit.prevent="send">
            <GlassInput
              v-model="input"
              :placeholder="editingMsg ? 'Изменить сообщение' : 'Сообщение'"
              class="flex-1"
              @input="onTyping"
              @keydown.enter.exact.prevent="send"
            />
            <FileUpload @files="onFiles">
              <template #default="{ open }">
                <GlassButton type="button" variant="secondary" class="px-3" @click="open">📎</GlassButton>
              </template>
            </FileUpload>
            <GlassButton
              type="button"
              variant="secondary"
              class="px-3 relative"
              @click="emojiOpen = !emojiOpen"
            >😀</GlassButton>
            <EmojiPicker :show="emojiOpen" @select="onEmoji" />
            <VoiceRecorder @send="onVoice" @cancel="() => {}" />
            <GlassButton type="submit" :disabled="!input.trim() && !attachments.length">Отправить</GlassButton>
          </form>
        </div>
      </div>
    </template>

    <div v-else class="flex-1 flex flex-col items-center justify-center gap-3">
      <div
        class="glass-soft w-16 h-16 rounded-3xl flex items-center justify-center text-2xl font-bold"
        :style="{ background: 'linear-gradient(135deg, rgba(124,108,240,0.9), rgba(45,212,191,0.85))', color: '#fff' }"
      >
        H
      </div>
      <p class="text-sm" :style="{ color: 'var(--color-text-muted)' }">Выберите чат</p>
    </div>

    <CallModal />
    <GlassModal :show="imagePreview" @close="imagePreview = false">
      <img :src="previewSrc" class="max-w-full max-h-[70vh] rounded-xl object-contain" />
    </GlassModal>

    <div
      v-if="menu.visible"
      class="fixed z-50 min-w-[160px] rounded-xl overflow-hidden text-sm glass-soft"
      :style="{ left: menu.x + 'px', top: menu.y + 'px', border: '1px solid rgba(255,255,255,0.1)' }"
      @click.stop
    >
      <button
        v-for="item in menu.items"
        :key="item.label"
        class="w-full text-left px-3 py-2.5 hover:bg-white/10 flex items-center gap-2"
        :class="{ 'text-red-400': item.danger }"
        @click="runMenu(item.action)"
      >
        {{ item.label }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from "vue"
import { useRoute } from "vue-router"
import { useChatStore, type Message, parseAttachment } from "../stores/chatStore"
import { useUserStore } from "../stores/userStore"
import { useCallStore } from "../stores/callStore"
import { wsClient } from "../services/ws"
import { showNativeNotification } from "../services/tauri"
import GlassInput from "./GlassInput.vue"
import GlassButton from "./GlassButton.vue"
import Avatar from "./Avatar.vue"
import EmojiPicker from "./EmojiPicker.vue"
import VoiceRecorder from "./VoiceRecorder.vue"
import VoicePlayer from "./VoicePlayer.vue"
import FileUpload from "./FileUpload.vue"
import FileMessage from "./FileMessage.vue"
import GlassModal from "./GlassModal.vue"
import CallModal from "./CallModal.vue"

const route = useRoute()
const chatStore = useChatStore()
const store = useUserStore()
const callStore = useCallStore()

const input = ref("")
const msgContainer = ref<HTMLElement | null>(null)
const emojiOpen = ref(false)
const imagePreview = ref(false)
const previewSrc = ref("")
const attachments = ref<{ file: File; name: string; preview?: string }[]>([])
const replyTo = ref<Message | null>(null)
const editingMsg = ref<Message | null>(null)
const menu = ref<{ visible: boolean; x: number; y: number; msg: Message | null; items: { label: string; action: string; danger?: boolean }[] }>({
  visible: false,
  x: 0,
  y: 0,
  msg: null,
  items: [],
})
let typingTimer: ReturnType<typeof setTimeout> | null = null
let readTimer: ReturnType<typeof setTimeout> | null = null
let lastReadId = ""

const typingNames = computed(() => {
  if (!chatStore.currentChat) return []
  return chatStore.typingIn(chatStore.currentChat.id)
})

onMounted(async () => {
  if (!store.user) await store.fetchMe()
  setupWS()
  callStore.setup()
  if (route.params.id) {
    await chatStore.fetchChats()
    const found = chatStore.chats.find((c) => c.id === route.params.id)
    if (found) chatStore.setCurrentChat(found)
    chatStore.fetchMessages(route.params.id as string)
  }
})

function setupWS() {
  wsClient.on("new_message", (payload: any) => {
    const msg = payload?.message
    if (!msg) return
    chatStore.upsertMessage(msg)
    if (chatStore.currentChat && msg.chat_id === chatStore.currentChat.id) {
      scheduleRead(msg)
    } else if (msg.sender_id !== store.user?.id) {
      const sender = msg.sender_name || "Новое сообщение"
      const text =
        msg.type === "voice"
          ? "Голосовое сообщение"
          : msg.type === "file"
            ? msg.file_name || "Файл"
            : (msg.text || "").slice(0, 120)
      showNativeNotification(sender, text)
    }
  })

  wsClient.on("read", (payload: any) => {
    if (payload?.chat_id && payload?.message_id) {
      chatStore.applyRead(payload.chat_id, payload.reader_id, payload.message_id)
    }
  })

  wsClient.on("delivered", (payload: any) => {
    if (payload?.chat_id && payload?.message_id) {
      chatStore.applyDelivered(payload.chat_id, payload.message_id)
    }
  })

  wsClient.on("message_updated", (payload: any) => {
    if (payload?.message) chatStore.applyMessageUpdated(payload.message)
  })

  wsClient.on("message_deleted", (payload: any) => {
    if (payload?.message_id) chatStore.applyMessageDeleted(payload.message_id)
  })

  wsClient.on("typing", (payload: any) => {
    if (payload?.chat_id && payload?.user_id) {
      chatStore.setTyping(payload.chat_id, payload.user_id, payload.is_typing)
      if (payload.is_typing) {
        setTimeout(() => chatStore.setTyping(payload.chat_id, payload.user_id, false), 3000)
      }
    }
  })
}

watch(
  () => route.params.id,
  (id) => {
    if (id) {
      chatStore.setCurrentChat({ id: id as string, title: "", type: "personal", avatar: "", updated_at: new Date().toISOString() })
      chatStore.fetchMessages(id as string)
    }
  },
)

watch(
  () => chatStore.messages.length,
  () => nextTick(() => {
    const el = msgContainer.value
    if (el) el.scrollTop = el.scrollHeight
  }),
)

function onScroll() {
  const el = msgContainer.value
  if (el && el.scrollTop < 80) {
    loadOlder()
  }
}

async function loadOlder() {
  if (!chatStore.currentChat || chatStore.loading || !chatStore.hasMore) return
  const el = msgContainer.value
  const prevHeight = el?.scrollHeight || 0
  const prevTop = el?.scrollTop || 0
  await chatStore.fetchMessages(chatStore.currentChat.id, true)
  await nextTick()
  if (el) {
    el.scrollTop = el.scrollHeight - prevHeight + prevTop
  }
}

function onTyping() {
  if (!chatStore.currentChat) return
  if (typingTimer) {
    clearTimeout(typingTimer)
    typingTimer = setTimeout(sendTypingOnce, 500)
  } else {
    typingTimer = setTimeout(sendTypingOnce, 0)
  }
}

function sendTypingOnce() {
  if (chatStore.currentChat) chatStore.sendTyping(chatStore.currentChat.id)
  typingTimer = null
}

function scheduleRead(msg: Message) {
  if (msg.sender_id === store.user?.id) return
  if (lastReadId === msg.id) return
  lastReadId = msg.id
  chatStore.clearUnread(msg.chat_id)
  if (readTimer) clearTimeout(readTimer)
  readTimer = setTimeout(() => {
    if (chatStore.currentChat) chatStore.markRead(chatStore.currentChat.id, lastReadId)
  }, 1000)
}

function send() {
  const content = input.value.trim()
  if (!content && !attachments.value.length) return
  if (!chatStore.currentChat) return
  if (editingMsg.value) {
    chatStore.editMessage(editingMsg.value.id, content)
    cancelAction()
    return
  }
  if (content) chatStore.sendMessage(chatStore.currentChat.id, content, "text", replyTo.value?.id || null)
  for (const a of attachments.value) {
    chatStore.sendFile(chatStore.currentChat.id, a.file, a.preview ? "image" : "file")
  }
  input.value = ""
  attachments.value = []
  emojiOpen.value = false
  replyTo.value = null
}

function cancelAction() {
  replyTo.value = null
  editingMsg.value = null
  input.value = ""
}

function openMenu(msg: Message, event: MouseEvent) {
  const items: { label: string; action: string; danger?: boolean }[] = [
    { label: "Ответить", action: "reply" },
    { label: "Копировать", action: "copy" },
  ]
  const isOwn = msg.sender_id === store.user?.id
  if (isOwn && isEditable(msg)) items.push({ label: "Изменить", action: "edit" })
  if (isOwn) items.push({ label: "Удалить", action: "delete", danger: true })
  menu.value = { visible: true, x: event.clientX, y: event.clientY, msg, items }
}

function runMenu(action: string) {
  const msg = menu.value.msg
  menu.value.visible = false
  if (!msg) return
  switch (action) {
    case "reply":
      replyTo.value = msg
      editingMsg.value = null
      input.value = ""
      break
    case "copy":
      navigator.clipboard?.writeText(msg.content).catch(() => {})
      break
    case "edit":
      if (msg.type !== "text") return
      editingMsg.value = msg
      replyTo.value = null
      input.value = msg.content
      break
    case "delete":
      if (confirm("Удалить сообщение?")) chatStore.deleteMessage(msg.id)
      break
  }
}

function isEditable(msg: Message) {
  return msg.type === "text" || msg.type === "file" || msg.type === "image"
}

function preview(msg: Message) {
  if (msg.type === "image") return "🖼 Фото"
  if (msg.type === "voice") return "🎤 Голосовое"
  if (msg.type === "file") return "📎 Файл"
  return msg.content
}

function replyPreview(msgId: string) {
  const replied = chatStore.messages.find((m) => m.id === msgId)
  return replied ? preview(replied) : "сообщение"
}

function onCloseMenu() {
  menu.value.visible = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") cancelAction()
}

onMounted(() => {
  window.addEventListener("click", onCloseMenu)
  window.addEventListener("keydown", onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener("click", onCloseMenu)
  window.removeEventListener("keydown", onKeydown)
})

function onEmoji(emoji: string) {
  input.value += emoji
  emojiOpen.value = false
}

function onFiles(files: File[]) {
  for (const f of files) {
    const isImg = f.type.startsWith("image/")
    attachments.value.push({
      file: f,
      name: f.name,
      preview: isImg ? URL.createObjectURL(f) : undefined,
    })
  }
}

function removeAttachment(i: number) {
  if (attachments.value[i]?.preview) URL.revokeObjectURL(attachments.value[i].preview!)
  attachments.value.splice(i, 1)
}

async function onVoice(blob: Blob, duration = 0, waveform: number[] = []) {
  if (!chatStore.currentChat) return
  await chatStore.sendVoice(chatStore.currentChat.id, blob, duration, waveform)
}

function callVideo() {
  if (!chatStore.currentChat) return
  const chat = chatStore.currentChat
  if (chat.type !== "personal") return
  const me = store.user?.id
  const memberIds = [chat.title] // заглушка — реально берём из members
  void memberIds
  // Получим собеседника через members чата.
  import("../services/api").then(async ({ api }) => {
    const res: any = await api.get(`/chat/${chat.id}/members`)
    const members: any[] = res.data || []
    const callee = members.find((m) => m.user_id !== me)
    if (callee) callStore.startCall(callee.user_id, "video")
  })
}

function previewImage(src: string) {
  previewSrc.value = src
  imagePreview.value = true
}

function isImage(msg: Message) {
  return msg.type === "image" || (msg.type === "text" && msg.content.startsWith("data:image"))
}

function isFile(msg: Message) {
  return msg.type === "file"
}

function isVoice(msg: Message) {
  return msg.type === "voice"
}

function attachmentName(msg: Message) {
  return parseAttachment(msg.content)?.name || "файл"
}

function attachmentSize(msg: Message) {
  return parseAttachment(msg.content)?.size || 0
}

function attachmentDuration(msg: Message) {
  return parseAttachment(msg.content)?.duration || 0
}

function attachmentWaveform(msg: Message) {
  return parseAttachment(msg.content)?.waveform || []
}

function hostname(url: string) {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

function statusIcon(status: string) {
  if (status === "read") return "✓✓✓"
  if (status === "delivered") return "✓✓"
  return "✓"
}

function statusColor(status: string) {
  if (status === "read") return "#2dd4bf"
  if (status === "delivered") return "#cdc6ff"
  return "rgba(255,255,255,0.5)"
}

function time(iso: string) {
  const d = new Date(iso)
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`
}
</script>
