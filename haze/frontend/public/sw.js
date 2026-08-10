/* Haze service worker — Web Push уведомления */
self.addEventListener("push", (event) => {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch {
    data = { title: "Haze", body: "Новое сообщение" }
  }

  const title = data.title || "Haze"
  const options = {
    body: data.body || "",
    icon: "/haze-icon.png",
    badge: "/haze-icon.png",
    data: data.data || {},
    tag: data.data?.chat_id || undefined,
  }

  event.waitUntil(self.registration.showNotification(title, options))
})

self.addEventListener("notificationclick", (event) => {
  event.notification.close()
  const chatId = event.notification.data?.chat_id
  const url = chatId ? `/chats/${chatId}` : "/chats"
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if ("focus" in client) {
          client.navigate(url)
          return client.focus()
        }
      }
      return self.clients.openWindow(url)
    }),
  )
})
