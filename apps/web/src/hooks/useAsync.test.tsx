import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useAsync } from './useAsync'

describe('useAsync', () => {
  it('loads data and exposes it', async () => {
    const { result } = renderHook(() => useAsync(() => Promise.resolve(42), []))
    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toBe(42)
    expect(result.current.error).toBeUndefined()
  })

  it('exposes errors', async () => {
    const { result } = renderHook(() => useAsync(() => Promise.reject(new Error('boom')), []))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error?.message).toBe('boom')
  })

  it('reloads on demand', async () => {
    const loader = vi.fn().mockResolvedValue('x')
    const { result } = renderHook(() => useAsync(loader, []))
    await waitFor(() => expect(result.current.loading).toBe(false))
    act(() => result.current.reload())
    await waitFor(() => expect(loader).toHaveBeenCalledTimes(2))
  })
})
