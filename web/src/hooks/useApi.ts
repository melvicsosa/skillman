import { useCallback, useEffect, useRef, useState } from 'react'

import { ApiError } from '../api/client'

export type ApiState<T> = {
  data: T | null
  error: string | null
  /** HTTP status of the last failure, null when none or not an API error. */
  status: number | null
  loading: boolean
  /** Re-run the fetch. Keeps the previous data visible while loading. */
  reload: () => Promise<void>
}

/**
 * Runs fn once on mount and exposes the result. fn must be stable (wrap it
 * in useCallback or pass a module-level function) or it will refetch on
 * every render.
 */
export function useApi<T>(fn: () => Promise<T>): ApiState<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const seq = useRef(0)

  const reload = useCallback(async () => {
    const id = ++seq.current
    setLoading(true)
    try {
      const result = await fn()
      if (id === seq.current) {
        setData(result)
        setError(null)
        setStatus(null)
      }
    } catch (err) {
      if (id === seq.current) {
        setError(err instanceof Error ? err.message : String(err))
        setStatus(err instanceof ApiError ? err.status : null)
      }
    } finally {
      if (id === seq.current) setLoading(false)
    }
  }, [fn])

  useEffect(() => {
    void reload()
  }, [reload])

  return { data, error, status, loading, reload }
}
