type MessageHandler = (data: any) => void

class WSClient {
  private ws: WebSocket | null = null
  private handlers = new Map<string, MessageHandler[]>()
  private url: string
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null

  constructor() {
    const protocol = location.protocol === "https:" ? "wss:" : "ws:"
    this.url = `${protocol}//${location.host}/api/chat/ws`
  }

  connect() {
    const token = localStorage.getItem("access_token")
    if (!token) return

    this.ws = new WebSocket(`${this.url}?token=${token}`)

    this.ws.onopen = () => {
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }
    }

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        const handlers = this.handlers.get(msg.type) || []
        handlers.forEach((h) => h(msg.payload))
      } catch {
        /* ignore */
      }
    }

    this.ws.onclose = () => {
      this.reconnectTimer = setTimeout(() => this.connect(), 3000)
    }

    this.ws.onerror = () => {
      this.ws?.close()
    }
  }

  disconnect() {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
    this.ws = null
  }

  on(type: string, handler: MessageHandler): () => void {
    if (!this.handlers.has(type)) this.handlers.set(type, [])
    this.handlers.get(type)!.push(handler)
    return () => this.off(type, handler)
  }

  off(type: string, handler: MessageHandler) {
    const handlers = this.handlers.get(type)
    if (handlers) {
      const idx = handlers.indexOf(handler)
      if (idx !== -1) handlers.splice(idx, 1)
    }
  }
}

export const wsClient = new WSClient()
