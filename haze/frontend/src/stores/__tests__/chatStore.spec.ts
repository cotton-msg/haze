import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { setActivePinia, createPinia } from "pinia"
import { useChatStore, parseAttachment, encodeAttachment, type Message, type Chat } from "../chatStore"
import { useUserStore, type User } from "../userStore"
import { api } from "../../services/api"

describe("parseAttachment", () => {
  it("parses valid JSON content", () => {
    const meta = parseAttachment('{"name":"a.png","size":10,"mime":"image/png"}')
    expect(meta).toEqual({ name: "a.png", size: 10, mime: "image/png" })
  })

  it("returns null for non-JSON content", () => {
    expect(parseAttachment("hello world")).toBeNull()
  })

  it("returns null for invalid JSON", () => {
    expect(parseAttachment("{not json")).toBeNull()
  })
})

describe("encodeAttachment", () => {
  it("encodes url and meta", () => {
    const out = encodeAttachment("http://cdn/x.png", { name: "x.png", size: 5 })
    expect(JSON.parse(out)).toEqual({ url: "http://cdn/x.png", name: "x.png", size: 5 })
  })
})

describe("chatStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const store = new Map<string, string>()
    ;(globalThis as any).localStorage = {
      getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
      clear: () => void store.clear(),
    }
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("fetches chats", async () => {
    const chats: Chat[] = [{ id: "c1", type: "personal", title: "Alice", avatar: "", updated_at: "x" }]
    vi.spyOn(api, "get").mockResolvedValue({ error: false, data: chats })

    const store = useChatStore()
    await store.fetchChats()
    expect(store.chats).toHaveLength(1)
    expect(store.chats[0].title).toBe("Alice")
  })

  it("fetches messages and disables hasMore when short page", async () => {
    vi.spyOn(api, "get").mockResolvedValue({ error: false, data: [] })

    const store = useChatStore()
    await store.fetchMessages("c1")
    expect(store.loading).toBe(false)
    expect(store.hasMore).toBe(false)
  })

  it("sends a message and upserts it", async () => {
    const msg: Message = { id: "m1", chat_id: "c1", sender_id: "me", content: "hi", type: "text", reply_to: null, status: "sent", created_at: "t" }
    vi.spyOn(api, "post").mockResolvedValue({ error: false, data: msg })

    const store = useChatStore()
    await store.sendMessage("c1", "hi")
    expect(store.messages.some((m) => m.id === "m1")).toBe(true)
  })

  it("upsertMessage replaces existing and updates chat preview", () => {
    const store = useChatStore()
    store.chats.push({ id: "c1", type: "group", title: "G", avatar: "", updated_at: "a" })

    const m1: Message = { id: "m1", chat_id: "c1", sender_id: "me", content: "hello", type: "text", reply_to: null, status: "sent", created_at: "t" }
    store.upsertMessage(m1)
    expect(store.messages).toHaveLength(1)
    expect(store.chats[0].last_message).toBe("hello")

    const m1b = { ...m1, content: "updated" }
    store.upsertMessage(m1b)
    expect(store.messages).toHaveLength(1)
    expect(store.messages[0].content).toBe("updated")
  })

  it("displayPreview maps media types", () => {
    const store = useChatStore()
    expect(store.upsertMessage).toBeDefined()
    const base: Message = { id: "m", chat_id: "c1", sender_id: "me", content: "x", type: "text", reply_to: null, status: "sent", created_at: "t" }

    store.chats.push({ id: "c1", type: "personal", title: "A", avatar: "", updated_at: "a" })
    store.upsertMessage({ ...base, type: "image" })
    expect(store.chats[0].last_message).toBe("🖼 Фото")
    store.upsertMessage({ ...base, type: "voice" })
    expect(store.chats[0].last_message).toBe("🎤 Голосовое")
    store.upsertMessage({ ...base, type: "file" })
    expect(store.chats[0].last_message).toBe("📎 Файл")
  })

  it("setTyping tracks users and typingIn excludes self", () => {
    const store = useChatStore()
    const me: User = { id: "me", username: "kiril", email: "k@k.k", display_name: "Kiril", avatar_url: "", bio: "", role: "user", is_premium: false }
    useUserStore().user = me
    store.setTyping("c1", "user-1", true)
    store.setTyping("c1", "me", true)
    expect(store.typingIn("c1")).toEqual(["user-1"])
    store.setTyping("c1", "user-1", false)
    expect(store.typingIn("c1")).toHaveLength(0)
  })

  it("setCurrentChat resets messages and unread", () => {
    const store = useChatStore()
    store.chats.push({ id: "c1", type: "group", title: "G", avatar: "", updated_at: "a", unread_count: 5 })
    store.messages.push({ id: "m1", chat_id: "c1", sender_id: "x", content: "y", type: "text", reply_to: null, status: "sent", created_at: "t" })

    store.setCurrentChat(store.chats[0])
    expect(store.messages).toHaveLength(0)
    expect(store.chats[0].unread_count).toBe(0)
  })

  it("markRead calls api", async () => {
    const spy = vi.spyOn(api, "post").mockResolvedValue({ error: false })
    const store = useChatStore()
    await store.markRead("c1", "m1")
    expect(spy).toHaveBeenCalledWith("/chat/c1/read", { message_id: "m1" })
  })
})
