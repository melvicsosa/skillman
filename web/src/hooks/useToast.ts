import { useCallback, useRef, useState } from 'react'

import type { ToastMessage } from '../components/molecules/Toast'

export type ToastFn = (title: string, body?: string, tone?: ToastMessage['tone']) => void

export function useToast(): { toasts: ToastMessage[]; toast: ToastFn } {
  const [toasts, setToasts] = useState<ToastMessage[]>([])
  const seq = useRef(0)
  const toast = useCallback<ToastFn>((title, body, tone = 'ok') => {
    const id = ++seq.current
    setToasts((t) => [...t, { id, title, body, tone }])
    window.setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), tone === 'error' ? 8000 : 5000)
  }, [])
  return { toasts, toast }
}

export function errMsg(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}
