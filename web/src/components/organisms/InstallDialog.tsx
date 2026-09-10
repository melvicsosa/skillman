import { useEffect, useState } from 'react'

import type { AddResult, Agent, InstallRequest, Project } from '../../api/client'
import { Alert } from '../atoms/Alert'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'
import { Spinner } from '../atoms/Spinner'
import { Select } from '../molecules/Select'

export type InstallTarget = {
  /** Ref passed to POST /api/registry/add. */
  ref: string
  /** Display name, defaults to ref. */
  name?: string
  source?: string
}

type Props = {
  target: InstallTarget
  agents: Agent[]
  projects: Project[]
  onSubmit: (ref: string, req: InstallRequest) => Promise<AddResult>
  onClose: () => void
}

/**
 * Modal used by Discover and by "Add by ref" in the Vault: pick agents,
 * optional project scope and copy mode, then show what was added.
 */
export function InstallDialog({ target, agents, projects, onSubmit, onClose }: Props) {
  const installable = agents.filter((a) => !a.readOnly)
  const [selected, setSelected] = useState<Set<string>>(
    () => new Set(installable.filter((a) => a.enabled).map((a) => a.id)),
  )
  const [project, setProject] = useState('')
  const [copy, setCopy] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<AddResult | null>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !busy) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [busy, onClose])

  const toggle = (id: string, on: boolean) =>
    setSelected((s) => {
      const next = new Set(s)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })

  const submit = async () => {
    setBusy(true)
    setError(null)
    try {
      const req: InstallRequest = { agents: [...selected], copy }
      if (project) req.project = project
      setResult(await onSubmit(target.ref, req))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={() => !busy && onClose()}>
      <div className="modal" role="dialog" aria-modal="true" aria-labelledby="install-title" onClick={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <h2 id="install-title">Install {target.name ?? target.ref}</h2>
          <span className="modal-sub mono" title={target.ref}>
            {target.source ? `${target.source} · ` : ''}
            {target.ref}
          </span>
        </div>
        {result ? (
          <AddSummary result={result} />
        ) : (
          <div className="modal-body">
            <fieldset className="field">
              <legend>Agents</legend>
              {installable.length === 0 && <p className="hint">No installable agents.</p>}
              <div className="check-grid">
                {installable.map((a) => (
                  <label key={a.id} className={`check${a.enabled ? '' : ' is-off'}`}>
                    <input
                      type="checkbox"
                      checked={selected.has(a.id)}
                      onChange={(e) => toggle(a.id, e.target.checked)}
                      disabled={busy}
                    />
                    {a.name}
                    {!a.enabled && <span className="hint"> (disabled)</span>}
                  </label>
                ))}
              </div>
            </fieldset>
            <div className="field">
              <span id="install-scope-label">Scope</span>
              <Select
                block
                labelledBy="install-scope-label"
                value={project}
                onChange={setProject}
                disabled={busy}
                options={[
                  { value: '', label: 'Global' },
                  ...projects.map((p) => ({ value: p.root, label: p.name, hint: p.root, title: p.root })),
                ]}
              />
            </div>
            <label className="check">
              <input type="checkbox" checked={copy} onChange={(e) => setCopy(e.target.checked)} disabled={busy} />
              Copy files instead of symlinking
            </label>
            {error && <Alert>{error}</Alert>}
          </div>
        )}
        <div className="modal-foot">
          {result ? (
            <Button variant="primary" onClick={onClose}>
              Done
            </Button>
          ) : (
            <>
              <Button variant="ghost" onClick={onClose} disabled={busy}>
                Cancel
              </Button>
              <Button variant="primary" onClick={submit} disabled={busy || selected.size === 0} aria-busy={busy}>
                {busy ? <Spinner /> : null}
                {busy ? 'Installing' : 'Install'}
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

function AddSummary({ result }: { result: AddResult }) {
  return (
    <div className="modal-body">
      <p>
        Added <strong>{result.entries.length}</strong> {result.entries.length === 1 ? 'skill' : 'skills'} to the vault
        {result.shape && (
          <>
            {' '}
            <Badge>{result.shape}</Badge>
          </>
        )}
      </p>
      <ul className="result-list">
        {result.entries.map((e) => (
          <li key={e.name}>
            <span className="skill-name">{e.name}</span>
            {e.convertedFrom && <Badge tone="info">{`converted from ${e.convertedFrom}`}</Badge>}
            {!e.spec.valid && <Badge tone="danger" title={(e.spec.issues ?? []).join('\n')}>spec issues</Badge>}
          </li>
        ))}
      </ul>
      {result.links.length > 0 && (
        <ul className="result-list">
          {result.links.map((l) => (
            <li key={`${l.agent}-${l.path}`}>
              <Badge>{l.agent}</Badge> <span className="mono">{l.path}</span>
              {l.existing && <Badge>already installed</Badge>}
              {l.copied && <Badge>copy</Badge>}
            </li>
          ))}
        </ul>
      )}
      {result.warnings.length > 0 && (
        <Alert tone="warn">
          <ul className="alert-list">
            {result.warnings.map((w, i) => (
              <li key={i}>{w}</li>
            ))}
          </ul>
        </Alert>
      )}
    </div>
  )
}
