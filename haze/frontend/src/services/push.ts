import { api } from "./api"

// Регистрирует service worker и подписку на Web Push.
export async function initPushNotifications(): Promise<void> {
  if (!("serviceWorker" in navigator) || !("PushManager" in window)) return
  if (!location.protocol.startsWith("https") && location.hostname !== "localhost") return

  try {
    const reg = await navigator.serviceWorker.register("/sw.js")
    await navigator.serviceWorker.ready

    const res: any = await api.get("/notifications/vapid")
    if (res.error || !res.data?.public_key) return

    const subscription = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(res.data.public_key),
    })

    const sub = subscription.toJSON()
    await api.post("/notifications/register", {
      endpoint: sub.endpoint,
      p256dh: sub.keys?.p256dh,
      auth: sub.keys?.auth,
    })
  } catch {
    /* push недоступен — не критично */
  }
}

// Web Push использует base64url для applicationServerKey.
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/")
  const rawData = atob(base64)
  const outputArray = new Uint8Array(rawData.length)
  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i)
  }
  return outputArray
}
