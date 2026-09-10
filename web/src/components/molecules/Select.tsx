import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'

import { Icon } from '../atoms/Icon'

export type SelectOption<T extends string> = {
  value: T
  label: string
  /** Secondary text shown muted after the label (e.g. a path). */
  hint?: string
  title?: string
}

type Props<T extends string> = {
  value: T
  options: SelectOption<T>[]
  onChange: (value: T) => void
  /** Accessible name; also used when no visible label is linked. */
  ariaLabel?: string
  /** id of an element labelling this control. */
  labelledBy?: string
  disabled?: boolean
  className?: string
  /** Stretch to the container width (forms). */
  block?: boolean
}

/** Styled replacement for the native <select>: trigger button plus a listbox
 * popover. Keyboard: ArrowUp/Down, Home/End, Enter/Space, Escape, Tab, type-ahead. */
export function Select<T extends string>({
  value,
  options,
  onChange,
  ariaLabel,
  labelledBy,
  disabled,
  className = '',
  block,
}: Props<T>) {
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const typed = useRef({ text: '', at: 0 })
  const id = useId()
  const listId = `${id}-list`

  const selectedIndex = Math.max(
    0,
    options.findIndex((o) => o.value === value),
  )
  const current = options.find((o) => o.value === value)

  const openList = useCallback(
    (index?: number) => {
      if (disabled || options.length === 0) return
      setActive(index ?? selectedIndex)
      setOpen(true)
    },
    [disabled, options.length, selectedIndex],
  )

  const close = useCallback((refocus: boolean) => {
    setOpen(false)
    if (refocus) triggerRef.current?.focus()
  }, [])

  const commit = (index: number) => {
    const opt = options[index]
    if (opt && opt.value !== value) onChange(opt.value)
    close(true)
  }

  // Close on outside pointer down.
  useEffect(() => {
    if (!open) return
    const onDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', onDown)
    return () => document.removeEventListener('pointerdown', onDown)
  }, [open])

  // Focus the list when it opens and keep the active option in view.
  useEffect(() => {
    if (open) listRef.current?.focus()
  }, [open])
  useEffect(() => {
    if (!open) return
    const el = listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)
    el?.scrollIntoView({ block: 'nearest' })
  }, [open, active])

  const typeAhead = (key: string): number | null => {
    const now = Date.now()
    const t = typed.current
    t.text = now - t.at > 600 ? key : t.text + key
    t.at = now
    const needle = t.text.toLowerCase()
    const start = open ? active : selectedIndex
    for (let i = 1; i <= options.length; i++) {
      const idx = (start + (t.text.length > 1 ? i - 1 : i)) % options.length
      if (options[idx].label.toLowerCase().startsWith(needle)) return idx
    }
    return null
  }

  const onTriggerKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
      e.preventDefault()
      openList(e.key === 'ArrowUp' ? Math.max(0, selectedIndex - 1) : selectedIndex)
    } else if (e.key.length === 1 && !e.metaKey && !e.ctrlKey && !e.altKey) {
      const idx = typeAhead(e.key)
      if (idx !== null && options[idx].value !== value) onChange(options[idx].value)
    }
  }

  const onListKey = (e: KeyboardEvent<HTMLUListElement>) => {
    const last = options.length - 1
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setActive((a) => Math.min(last, a + 1))
        break
      case 'ArrowUp':
        e.preventDefault()
        setActive((a) => Math.max(0, a - 1))
        break
      case 'Home':
      case 'PageUp':
        e.preventDefault()
        setActive(0)
        break
      case 'End':
      case 'PageDown':
        e.preventDefault()
        setActive(last)
        break
      case 'Enter':
      case ' ':
        e.preventDefault()
        commit(active)
        break
      case 'Escape':
        e.preventDefault()
        e.stopPropagation()
        close(true)
        break
      case 'Tab':
        setOpen(false)
        break
      default:
        if (e.key.length === 1 && !e.metaKey && !e.ctrlKey && !e.altKey) {
          const idx = typeAhead(e.key)
          if (idx !== null) setActive(idx)
        }
    }
  }

  const cls = ['select', block ? 'is-block' : '', open ? 'is-open' : '', className].filter(Boolean).join(' ')
  return (
    <div className={cls} ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className="select-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-label={labelledBy ? undefined : ariaLabel}
        aria-labelledby={labelledBy ? `${labelledBy} ${id}-value` : undefined}
        disabled={disabled}
        title={current?.title}
        onClick={() => (open ? close(false) : openList())}
        onKeyDown={onTriggerKey}
      >
        <span className="select-value" id={`${id}-value`}>
          {current?.label ?? ''}
          {current?.hint && <span className="select-hint"> {current.hint}</span>}
        </span>
        <Icon name="chevron" size={14} className="select-chevron" />
      </button>
      {open && (
        <ul
          ref={listRef}
          id={listId}
          className="select-list"
          role="listbox"
          tabIndex={-1}
          aria-label={labelledBy ? undefined : ariaLabel}
          aria-labelledby={labelledBy}
          aria-activedescendant={`${id}-opt-${active}`}
          onKeyDown={onListKey}
          onBlur={(e) => {
            if (!rootRef.current?.contains(e.relatedTarget as Node)) setOpen(false)
          }}
        >
          {options.map((o, i) => {
            const selected = o.value === value
            return (
              <li
                key={o.value}
                id={`${id}-opt-${i}`}
                data-index={i}
                role="option"
                aria-selected={selected}
                className={`select-option${i === active ? ' is-active' : ''}${selected ? ' is-selected' : ''}`}
                title={o.title}
                onPointerMove={() => setActive(i)}
                onClick={() => commit(i)}
              >
                <span className="select-check">{selected && <Icon name="check" size={14} />}</span>
                <span className="select-option-text">
                  {o.label}
                  {o.hint && <span className="select-hint"> {o.hint}</span>}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
