import { useEffect, useRef, useState } from 'react'

import type { Project } from '../../api/client'
import { errMsg } from '../../hooks/useToast'
import { Alert } from '../atoms/Alert'
import { Button } from '../atoms/Button'
import { Icon } from '../atoms/Icon'
import { Spinner } from '../atoms/Spinner'
import { InlineConfirm } from './InlineConfirm'

type Props = {
  projects: Project[]
  /** Registers a root; rejects with the API error message on failure. */
  onAdd: (root: string) => Promise<void>
  onRemove: (project: Project) => Promise<void>
}

/** "Add project" button with a popover: path form plus a manage list. */
export function ProjectManager({ projects, onAdd, onRemove }: Props) {
  const [open, setOpen] = useState(false)
  const [root, setRoot] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [confirming, setConfirming] = useState<number | null>(null)
  const [removing, setRemoving] = useState<number | null>(null)
  const wrapRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const close = (refocus: boolean) => {
    setOpen(false)
    setRoot('')
    setError(null)
    setConfirming(null)
    if (refocus) buttonRef.current?.focus()
  }

  useEffect(() => {
    if (!open) return
    inputRef.current?.focus()
    const onDown = (e: PointerEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) {
        setOpen(false)
        setError(null)
        setConfirming(null)
      }
    }
    document.addEventListener('pointerdown', onDown)
    return () => document.removeEventListener('pointerdown', onDown)
  }, [open])

  const submit = async () => {
    const r = root.trim()
    if (!r) return
    setAdding(true)
    setError(null)
    try {
      await onAdd(r)
      close(true)
    } catch (err) {
      setError(errMsg(err))
    } finally {
      setAdding(false)
    }
  }

  const remove = async (p: Project) => {
    setRemoving(p.id)
    setError(null)
    try {
      await onRemove(p)
      setConfirming(null)
    } catch (err) {
      setError(errMsg(err))
    } finally {
      setRemoving(null)
    }
  }

  return (
    <div
      className="popover-wrap"
      ref={wrapRef}
      onKeyDown={(e) => {
        if (e.key === 'Escape' && open) {
          e.stopPropagation()
          close(true)
        }
      }}
    >
      <Button
        ref={buttonRef}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => (open ? close(false) : setOpen(true))}
      >
        <Icon name="plus" size={14} />
        Add project
      </Button>
      {open && (
        <div className="popover" role="dialog" aria-label="Projects">
          <form
            className="popover-form"
            onSubmit={(e) => {
              e.preventDefault()
              void submit()
            }}
          >
            <label className="field">
              <span>Project root</span>
              <input
                ref={inputRef}
                className="input"
                value={root}
                onChange={(e) => setRoot(e.target.value)}
                placeholder="/absolute/path/to/project"
                spellCheck={false}
                disabled={adding}
              />
            </label>
            <div className="popover-actions">
              <Button variant="ghost" onClick={() => close(true)} disabled={adding}>
                Cancel
              </Button>
              <Button type="submit" variant="primary" disabled={adding || !root.trim()} aria-busy={adding}>
                {adding ? <Spinner /> : null}
                Add
              </Button>
            </div>
          </form>
          {error && <Alert onDismiss={() => setError(null)}>{error}</Alert>}
          {projects.length > 0 && (
            <div className="popover-section">
              <div className="popover-label">Registered projects</div>
              <ul className="project-list">
                {projects.map((p) => (
                  <li key={p.id} className="project-row">
                    {confirming === p.id ? (
                      <InlineConfirm
                        message={`Remove ${p.name}? Files are not touched.`}
                        confirmLabel="Remove"
                        busy={removing === p.id}
                        onConfirm={() => void remove(p)}
                        onCancel={() => setConfirming(null)}
                      />
                    ) : (
                      <>
                        <span className="project-row-text" title={p.root}>
                          <span className="project-row-name">{p.name}</span>
                          <span className="project-row-root">{p.root}</span>
                        </span>
                        <button
                          type="button"
                          className="icon-btn"
                          aria-label={`Remove ${p.name}`}
                          title="Remove project"
                          onClick={() => setConfirming(p.id)}
                        >
                          <Icon name="close" size={14} />
                        </button>
                      </>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
