export type ToastMessage = {
  id: number
  title: string
  body?: string
  tone: 'ok' | 'error'
}

type Props = {
  toasts: ToastMessage[]
}

export function ToastArea({ toasts }: Props) {
  if (toasts.length === 0) return null
  return (
    <div className="toast-area" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={`toast${t.tone === 'error' ? ' is-error' : ''}`} role="status">
          <div className="toast-title">{t.title}</div>
          {t.body && <div className="toast-body">{t.body}</div>}
        </div>
      ))}
    </div>
  )
}
