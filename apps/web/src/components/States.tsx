import type { ReactNode } from 'react'
import type { AsyncState } from '../hooks/useAsync'

export function Loading() {
  return <p className="py-6 text-center text-sm text-slate-500">Loading…</p>
}

export function ErrorState({ error, onRetry }: { error: Error; onRetry?: () => void }) {
  return (
    <div role="alert" className="rounded border border-red-200 bg-red-50 p-4 text-sm text-red-800">
      <p>{error.message}</p>
      {onRetry && (
        <button type="button" className="btn-secondary mt-2" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  )
}

export function EmptyState({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="rounded border border-dashed border-slate-300 p-6 text-center">
      <p className="font-medium">{title}</p>
      {hint && <p className="mt-1 text-sm text-slate-500">{hint}</p>}
    </div>
  )
}

interface AsyncProps<T> {
  state: AsyncState<T>
  children: (data: T) => ReactNode
  isEmpty?: (data: T) => boolean
  empty?: ReactNode
}

/** Renders loading, error, empty and success states for an async loader. */
export function Async<T>({ state, children, isEmpty, empty }: AsyncProps<T>) {
  if (state.error) return <ErrorState error={state.error} onRetry={state.reload} />
  if (state.loading && state.data === undefined) return <Loading />
  if (state.data === undefined) return null
  if (isEmpty && isEmpty(state.data)) return <>{empty}</>
  return <>{children(state.data)}</>
}
