const BASE = "/api"

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem("access_token")
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  }
  if (token) headers["Authorization"] = `Bearer ${token}`

  const res = await fetch(`${BASE}${path}`, { ...options, headers })

  if (res.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      headers["Authorization"] = `Bearer ${localStorage.getItem("access_token")}`
      const retry = await fetch(`${BASE}${path}`, { ...options, headers })
      return retry.json()
    }
    localStorage.clear()
    window.location.href = "/login"
    throw new Error("unauthorized")
  }

  return res.json()
}

async function tryRefresh(): Promise<boolean> {
  const rt = localStorage.getItem("refresh_token")
  if (!rt) return false
  try {
    const res = await fetch(`${BASE}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: rt }),
    })
    if (!res.ok) return false
    const data = await res.json()
    if (data.data) {
      localStorage.setItem("access_token", data.data.access_token)
      localStorage.setItem("refresh_token", data.data.refresh_token)
      return true
    }
    return false
  } catch {
    return false
  }
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
}
