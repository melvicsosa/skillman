import { useId, type ReactNode } from 'react'

import { Icon } from '../atoms/Icon'

type Props = {
  /** Header content; rendered inside a full-width button. */
  summary: ReactNode
  open: boolean
  onToggle: () => void
  children: ReactNode
  /** Extra class on the wrapper, e.g. to tune density per context. */
  className?: string
  /** Mount the panel only while open (default). Set false to keep it in the DOM hidden. */
  unmountOnClose?: boolean
}

/**
 * Button header with a rotating chevron and a linked content region.
 * Controlled: the parent owns `open`, which lets a list enforce one-open-at-a-time.
 * Native button semantics give Enter/Space and the global focus ring for free.
 */
export function Disclosure({ summary, open, onToggle, children, className = '', unmountOnClose = true }: Props) {
  const id = useId()
  const panelId = `${id}-panel`
  const showPanel = open || !unmountOnClose
  return (
    <div className={`disclosure${open ? ' is-open' : ''} ${className}`.trim()}>
      <button type="button" className="disclosure-head" aria-expanded={open} aria-controls={panelId} onClick={onToggle}>
        <Icon name="chevron" size={14} className="disclosure-chevron" />
        <span className="disclosure-summary">{summary}</span>
      </button>
      {showPanel && (
        <div id={panelId} className="disclosure-panel" hidden={!open}>
          {children}
        </div>
      )}
    </div>
  )
}
