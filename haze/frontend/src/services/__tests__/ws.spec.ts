import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { wsClient } from "../ws"

class MockWebSocket {
  static instances: MockWebSocket[] = []
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  readyState = MockWebSocket.CONNECTING
  url: string

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }
}

describe("wsClient", () => {
  let originalWebSocket: typeof WebSocket
  let originalLocalStorage: Storage | undefined

  beforeEach(() => {
    originalWebSocket = globalThis.WebSocket
    ;(globalThis as any).WebSocket = MockWebSocket
    originalLocalStorage = (globalThis as any).localStorage

    const store = new Map<string, string>()
    const storage = {
      getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
      clear: () => void store.clear(),
      key: (i: number) => Array.from(store.keys())[i] ?? null,
      get length() {
        return store.size
      },
    } as unknown as Storage
    ;(globalThis as any).localStorage = storage

    MockWebSocket.instances = []
    window.localStorage.setItem("access_token", "test-token")
  })

  afterEach(() => {
    globalThis.WebSocket = originalWebSocket
    if (originalLocalStorage) (globalThis as any).localStorage = originalLocalStorage
    wsClient.disconnect()
    vi.restoreAllMocks()
  })

  it("connects with token when present", () => {
    wsClient.connect()
    expect(MockWebSocket.instances).toHaveLength(1)
    expect(MockWebSocket.instances[0].url).toContain("?token=test-token")
  })

  it("skips connect without token", () => {
    window.localStorage.removeItem("access_token")
    wsClient.connect()
    expect(MockWebSocket.instances).toHaveLength(0)
  })

  it("dispatches messages to registered handlers", () => {
    wsClient.connect()
    const handler = vi.fn()
    wsClient.on("message", handler)

    const ws = MockWebSocket.instances[0]
    ws.onmessage?.({ data: JSON.stringify({ type: "message", payload: { id: "1" } }) })

    expect(handler).toHaveBeenCalledWith({ id: "1" })
  })

  it("ignores invalid JSON", () => {
    wsClient.connect()
    const handler = vi.fn()
    wsClient.on("message", handler)

    MockWebSocket.instances[0].onmessage?.({ data: "{not json" })
    expect(handler).not.toHaveBeenCalled()
  })

  it("removes handlers with off", () => {
    wsClient.connect()
    const handler = vi.fn()
    wsClient.on("message", handler)
    wsClient.off("message", handler)

    MockWebSocket.instances[0].onmessage?.({ data: JSON.stringify({ type: "message" }) })
    expect(handler).not.toHaveBeenCalled()
  })

  it("disconnect closes socket and clears timer", () => {
    vi.useFakeTimers()
    wsClient.connect()
    const ws = MockWebSocket.instances[0]
    const closeSpy = vi.spyOn(ws, "close")

    wsClient.disconnect()
    expect(closeSpy).toHaveBeenCalled()
    vi.useRealTimers()
  })
})
