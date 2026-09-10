import { useState, type FormEvent } from 'react'

import type { ExportRequest } from '../../api/client'
import { Button } from '../atoms/Button'
import { Spinner } from '../atoms/Spinner'

type Props = {
  /** Vault entry names currently selected, in table order. */
  selected: string[]
  busy: boolean
  onExport: (req: ExportRequest) => Promise<void>
  onClear: () => void
}

/** Inline form shown above the vault table once at least one row is selected:
 * bundles the selected skills as a Claude plugin directory. */
export function ExportBar({ selected, busy, onExport, onClear }: Props) {
  const [outDir, setOutDir] = useState('')
  /** Null until the user edits the name; the first selection is the default. */
  const [nameOverride, setNameOverride] = useState<string | null>(null)
  const [version, setVersion] = useState('0.1.0')
  const [description, setDescription] = useState('')

  const name = nameOverride ?? selected[0] ?? ''

  const valid = outDir.trim() !== '' && name.trim() !== '' && selected.length > 0

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!valid || busy) return
    const req: ExportRequest = { outDir: outDir.trim(), name: name.trim(), skills: selected }
    if (version.trim()) req.version = version.trim()
    if (description.trim()) req.description = description.trim()
    await onExport(req)
  }

  return (
    <form className="selection-bar" onSubmit={submit} aria-label="Export as Claude plugin">
      <div className="selection-bar-head">
        <strong>{selected.length}</strong> {selected.length === 1 ? 'skill' : 'skills'} selected · Export as Claude plugin
      </div>
      <div className="selection-bar-fields">
        <label className="selection-field">
          <span>Output dir</span>
          <input
            className="input"
            value={outDir}
            onChange={(e) => setOutDir(e.target.value)}
            placeholder="/path/to/plugins"
            required
            disabled={busy}
          />
        </label>
        <label className="selection-field">
          <span>Plugin name</span>
          <input
            className="input is-short"
            value={name}
            onChange={(e) => setNameOverride(e.target.value)}
            required
            disabled={busy}
          />
        </label>
        <label className="selection-field">
          <span>Version</span>
          <input className="input is-short" value={version} onChange={(e) => setVersion(e.target.value)} placeholder="0.1.0" disabled={busy} />
        </label>
        <label className="selection-field">
          <span>Description</span>
          <input
            className="input"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional"
            disabled={busy}
          />
        </label>
      </div>
      <div className="selection-bar-actions">
        <Button variant="primary" type="submit" disabled={!valid || busy} aria-busy={busy}>
          {busy ? <Spinner /> : null}
          Export
        </Button>
        <Button variant="ghost" onClick={onClear} disabled={busy}>
          Clear selection
        </Button>
      </div>
    </form>
  )
}
