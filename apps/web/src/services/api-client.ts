const TOKEN_KEY = 'compatlab.token'

export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export function getToken(): string | null {
  return sessionStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null): void {
  if (token) sessionStorage.setItem(TOKEN_KEY, token)
  else sessionStorage.removeItem(TOKEN_KEY)
}

export const UNAUTHORIZED_EVENT = 'compatlab:unauthorized'

/** Sends a JSON request to the backend. Every dashboard number comes through here. */
export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  const res = await fetch(`/api/v1${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }
  if (res.status === 401 && token) {
    setToken(null)
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
  }
  if (!res.ok) {
    const message = (data as { error?: string } | null)?.error ?? res.statusText ?? 'Request failed'
    throw new ApiError(res.status, message)
  }
  return data as T
}

export const get = <T>(path: string) => request<T>('GET', path)
export const post = <T>(path: string, body: unknown) => request<T>('POST', path, body)
