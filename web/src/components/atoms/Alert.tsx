import type { ReactNode } from 'react'

type Tone = 'error' | 'warn' | 'info'

type Props = {
  tone?: Tone
  children: ReactNode
  /** Extra controls rendered on the right (retry, dismiss, ...). */
  actions?: ReactNode
  onDismiss?: () => void
}

/** Small inline notice for API errors and notes, shared across views. */
export function Alert({ tone = 'error', children, actions, onDismiss }: Props) {
  return (
    <div className={`alert is-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      <div className="alert-body">{children}</div>
      {(actions || onDismiss) && (
        <div className="alert-actions">
          {actions}
          {onDismiss && (
            <button type="button" className="btn is-ghost" onClick={onDismiss} aria-label="Dismiss">
              ×
            </button>
          )}
        </div>
      )}
    </div>
  )
}
