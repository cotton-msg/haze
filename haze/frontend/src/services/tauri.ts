import { isTauri, invoke } from "@tauri-apps/api/core"

export function inTauri(): boolean {
  return isTauri()
}

export async function showNativeNotification(title: string, body: string): Promise<void> {
  if (!isTauri()) return
  try {
    await invoke("show_notification", { title, body })
  } catch {
    /* нативные уведомления недоступны */
  }
}

export async function hideToTray(): Promise<void> {
  if (!isTauri()) return
  try {
    await invoke("hide_window")
  } catch {
    /* ignore */
  }
}

export async function restoreWindow(): Promise<void> {
  if (!isTauri()) return
  try {
    await invoke("show_window")
  } catch {
    /* ignore */
  }
}

export async function checkForUpdates(): Promise<void> {
  if (!isTauri()) return
  try {
    const res: any = await invoke("check_for_updates")
    if (res?.status === "installed") {
      await showNativeNotification("Haze", `Обновление ${res.version} установлено`)
    }
  } catch {
    /* обновления недоступны — не критично */
  }
}
