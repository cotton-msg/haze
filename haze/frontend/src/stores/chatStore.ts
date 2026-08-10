import { defineStore } from "pinia"
import { ref, computed } from "vue"
import { api } from "../services/api"
import { useUserStore } from "./userStore"

export interface Chat {
  id: string
  type: "personal" | "group" | "channel"
  title: string
  avatar: string
  last_message?: string
  unread_count?: number
  updated_at: string
}

export interface Message {
  id: string
  chat_id: string
  sender_id: string
  content: string
  type: string
  reply_to: string | null
  status: string
  edited_at?: string | null
  created_at: string
  link_preview?: {
    url: string
    title: string
    description: string
    image: string
  } | null
}

// Метаданные вложения, закодированные в content как JSON.
export interface AttachmentMeta {
  name?: string
  size?: number
  mime?: string
  duration?: number
  waveform?: number[]
}

export function parseAttachment(content: string): AttachmentMeta | null {
  if (!content.startsWith("{")) return null
  try {
    const obj = JSON.parse(content)
    if (typeof obj === "object" && obj !== null) return obj as AttachmentMeta
  } catch {
    /* ignore */
  }
  return null
}

export function encodeAttachment(url: string, meta: AttachmentMeta): string {
  return JSON.stringify({ url, ...meta })
}

export const useChatStore = defineStore("chat", () => {
  const chats = ref<Chat[]>([])
  const currentChat = ref<Chat | null>(null)
  // API контракт: /messages возвращает DESC (новые первыми).
  const messages = ref<Message[]>([])
  const loading = ref(false)
  const hasMore = ref(true)
  const pageSize = 30
  const typingUsers = ref<Record<string, string>>({})

  const messagesChrono = computed(() => [...messages.value].reverse())

  async function fetchChats() {
    try {
      const res: any = await api.get("/chat/list")
      if (!res.error) chats.value = res.data || []
    } catch {
      /* ignore */
    }
  }

  async function fetchMessages(chatId: string, append = false) {
    if (loading.value || !hasMore.value) return
    loading.value = true
    try {
      const offset = append ? messages.value.length : 0
      const res: any = await api.get(`/chat/${chatId}/messages?limit=${pageSize}&offset=${offset}`)
      if (!res.error) {
        const incoming: Message[] = res.data || []
        if (append) {
          messages.value = dedupe([...messages.value, ...incoming])
        } else {
          messages.value = incoming
        }
        if (incoming.length < pageSize) hasMore.value = false
      }
    } catch {
      /* ignore */
    } finally {
      loading.value = false
    }
  }

  async function sendMessage(chatId: string, content: string, type = "text", replyTo: string | null = null) {
    try {
      const res: any = await api.post(`/chat/${chatId}/message`, { content, type, reply_to: replyTo })
      if (!res.error && res.data) {
        upsertMessage(res.data)
      }
    } catch {
      /* ignore */
    }
  }

  async function editMessage(msgId: string, content: string) {
    try {
      const res: any = await api.put(`/chat/message/${msgId}`, { content })
      if (!res.error && res.data) {
        upsertMessage(res.data)
      }
    } catch {
      /* ignore */
    }
  }

  async function deleteMessage(msgId: string) {
    try {
      const res: any = await api.delete(`/chat/message/${msgId}`)
      if (!res.error) {
        messages.value = messages.value.filter((m) => m.id !== msgId)
      }
    } catch {
      /* ignore */
    }
  }

  // WS: сообщение изменено — обновить локально.
  function applyMessageUpdated(msg: Message) {
    const idx = messages.value.findIndex((m) => m.id === msg.id)
    if (idx !== -1) messages.value[idx] = msg
  }

  // WS: сообщение удалено — убрать локально.
  function applyMessageDeleted(msgId: string) {
    messages.value = messages.value.filter((m) => m.id !== msgId)
  }

  // Отправка файла: загрузка в media + сообщение с JSON-метаданными.
  async function sendFile(chatId: string, file: File, type = "file") {
    const upload = await uploadMedia(file)
    if (!upload) return
    const content = encodeAttachment(upload.url, { name: file.name, size: file.size, mime: file.type })
    await sendMessage(chatId, content, type)
  }

  async function sendVoice(chatId: string, blob: Blob, duration = 0, waveform: number[] = []) {
    const file = new File([blob], "voice.webm", { type: "audio/webm" })
    const upload = await uploadMedia(file)
    if (!upload) return
    const content = encodeAttachment(upload.url, { name: "voice.webm", size: file.size, mime: file.type, duration, waveform })
    await sendMessage(chatId, content, "voice")
  }

  async function uploadMedia(file: File): Promise<{ url: string } | null> {
    try {
      const token = localStorage.getItem("access_token")
      const form = new FormData()
      form.append("file", file)
      const res = await fetch("/api/media/upload", {
        method: "POST",
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body: form,
      })
      const data = await res.json()
      if (!data.error && data.data) return data.data
      return null
    } catch {
      return null
    }
  }

  async function markRead(chatId: string, messageId: string) {
    try {
      await api.post(`/chat/${chatId}/read`, { message_id: messageId })
    } catch {
      /* ignore */
    }
  }

  async function sendTyping(chatId: string) {
    try {
      await api.post(`/chat/${chatId}/typing`)
    } catch {
      /* ignore */
    }
  }

  // Обновление статуса своих сообщений из WS-события read.
  function applyRead(chatId: string, readerId: string, upToMessageId: string) {
    const me = useUserStore().user?.id
    if (!me || readerId === me) return
    const upTo = messages.value.find((m) => m.id === upToMessageId)
    const cutoff = upTo ? new Date(upTo.created_at).getTime() : Date.now()
    for (const m of messages.value) {
      if (m.chat_id === chatId && m.sender_id === me && m.status !== "read") {
        if (new Date(m.created_at).getTime() <= cutoff) m.status = "read"
      }
    }
  }

  // Обновление статуса своих сообщений из WS-события delivered.
  function applyDelivered(chatId: string, upToMessageId: string) {
    const me = useUserStore().user?.id
    if (!me) return
    const upTo = messages.value.find((m) => m.id === upToMessageId)
    const cutoff = upTo ? new Date(upTo.created_at).getTime() : Date.now()
    for (const m of messages.value) {
      if (m.chat_id === chatId && m.sender_id === me && m.status === "sent") {
        if (new Date(m.created_at).getTime() <= cutoff) m.status = "delivered"
      }
    }
  }

  function upsertMessage(msg: Message) {
    const idx = messages.value.findIndex((m) => m.id === msg.id)
    if (idx !== -1) {
      messages.value[idx] = msg
    } else {
      messages.value.unshift(msg)
    }

    // Обновляем превью и счётчик в списке чатов.
    const chat = chats.value.find((c) => c.id === msg.chat_id)
    if (chat) {
      chat.last_message = displayPreview(msg)
      chat.updated_at = msg.created_at
      if (msg.sender_id !== useUserStore().user?.id) {
        chat.unread_count = (chat.unread_count || 0) + 1
      }
    }
  }

  function displayPreview(msg: Message): string {
    switch (msg.type) {
      case "image":
        return "🖼 Фото"
      case "voice":
        return "🎤 Голосовое"
      case "file":
        return "📎 Файл"
      default:
        return msg.content
    }
  }

  function setTyping(chatId: string, userId: string, isTyping: boolean) {
    const key = `${chatId}:${userId}`
    if (isTyping) {
      typingUsers.value[key] = userId
    } else {
      delete typingUsers.value[key]
    }
  }

  // Участники чата, которые сейчас печатают (исключая себя).
  function typingIn(chatId: string): string[] {
    const me = useUserStore().user?.id
    const prefix = `${chatId}:`
    const result: string[] = []
    for (const [key, uid] of Object.entries(typingUsers.value)) {
      if (key.startsWith(prefix) && uid !== me) result.push(uid)
    }
    return result
  }

  function setCurrentChat(chat: Chat) {
    currentChat.value = chat
    messages.value = []
    hasMore.value = true
    clearUnread(chat.id)
  }

  function clearUnread(chatId: string) {
    const chat = chats.value.find((c) => c.id === chatId)
    if (chat && chat.unread_count) chat.unread_count = 0
  }

  return {
    chats,
    currentChat,
    messages,
    messagesChrono,
    loading,
    hasMore,
    typingUsers,
    fetchChats,
    fetchMessages,
    sendMessage,
    sendFile,
    sendVoice,
    editMessage,
    deleteMessage,
    applyMessageUpdated,
    applyMessageDeleted,
    markRead,
    sendTyping,
    applyRead,
    applyDelivered,
    upsertMessage,
    setTyping,
    typingIn,
    setCurrentChat,
    clearUnread,
  }
})

function dedupe(list: Message[]): Message[] {
  const seen = new Set<string>()
  const out: Message[] = []
  for (const item of list) {
    if (!seen.has(item.id)) {
      seen.add(item.id)
      out.push(item)
    }
  }
  return out
}
