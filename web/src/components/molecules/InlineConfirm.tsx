import type { ReactNode } from 'react'

import { Button } from '../atoms/Button'

type Props = {
  message: ReactNode
  confirmLabel: string
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

/** Confirmation rendered in place, so no browser dialogs are needed. */
export function InlineConfirm({ message, confirmLabel, busy, onConfirm, onCancel }: Props) {
  return (
    <span className="inline-confirm">
      <span className="inline-confirm-msg">{message}</span>
      <Button className="is-danger" onClick={onConfirm} disabled={busy} aria-busy={busy}>
        {confirmLabel}
      </Button>
      <Button variant="ghost" onClick={onCancel} disabled={busy}>
        Cancel
      </Button>
    </span>
  )
}
