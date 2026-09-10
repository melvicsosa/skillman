import type { ReactNode } from 'react'

type Props = {
  title: ReactNode
  hint?: ReactNode
}

/** Centered empty-state card with the brand icon, a title and an optional hint. */
export function EmptyState({ title, hint }: Props) {
  return (
    <div className="state-box is-empty">
      <img className="state-icon" src="/icon-512.png" width={56} height={56} alt="" aria-hidden="true" />
      <h3>{title}</h3>
      {hint && <p>{hint}</p>}
    </div>
  )
}
