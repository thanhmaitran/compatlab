import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, UNAUTHORIZED_EVENT, getToken, request, setToken } from './api-client'

function respond(status: number, body: unknown) {
  return Promise.resolve(
    new Response(body === null ? null : JSON.stringify(body), { status }),
  )
}

describe('api client', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends the bearer token and JSON body, and parses the response', async () => {
    setToken('abc')
    const fetchMock = vi.fn().mockReturnValue(respond(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)

    const out = await request<{ ok: boolean }>('POST', '/projects', { name: 'x' })

    expect(out).toEqual({ ok: true })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/projects')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer abc')
    expect(init.body).toBe(JSON.stringify({ name: 'x' }))
  })

  it('throws an ApiError carrying the server message', async () => {
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(respond(409, { error: 'already exists' })))

    const result = request('POST', '/projects', {})

    await expect(result).rejects.toBeInstanceOf(ApiError)
    await expect(result).rejects.toMatchObject({
      status: 409,
      message: 'already exists',
    })
  })

  it('clears the token and notifies the app on 401', async () => {
    setToken('stale')
    const listener = vi.fn()
    window.addEventListener(UNAUTHORIZED_EVENT, listener)
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(respond(401, { error: 'invalid or expired token' })))

    await expect(request('GET', '/projects')).rejects.toBeInstanceOf(ApiError)

    expect(getToken()).toBeNull()
    expect(listener).toHaveBeenCalledOnce()
    window.removeEventListener(UNAUTHORIZED_EVENT, listener)
  })

  it('handles empty bodies', async () => {
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(respond(204, null)))

    await expect(request('GET', '/health')).resolves.toBeNull()
  })
})
