import { useState } from 'react'

import { ApiError, type Agent, type VaultEntry } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'
import { Checkbox } from '../atoms/Checkbox'
import { Spinner } from '../atoms/Spinner'
import { Toggle } from '../atoms/Toggle'
import { InlineConfirm } from './InlineConfirm'

type Props = {
  entry: VaultEntry
  agents: Agent[]
  /** Project root when the matrix shows project scope, null for global. */
  project: string | null
  /** "name/agent" of the cell currently being changed, or name for row actions. */
  busy: string | null
  selected: boolean
  onSelect: (entry: VaultEntry, selected: boolean) => void
  onToggle: (entry: VaultEntry, agent: Agent, installed: boolean) => Promise<void>
  onUpdate: (entry: VaultEntry) => Promise<void>
  onSync: (entry: VaultEntry) => Promise<void>
  onAutoSync: (entry: VaultEntry, autoSync: boolean) => Promise<void>
  onRemove: (entry: VaultEntry, force: boolean) => Promise<void>
}

type RemoveState = { step: 'idle' } | { step: 'confirm' } | { step: 'linked'; message: string }

export function VaultRow({ entry, agents, project, busy, selected, onSelect, onToggle, onUpdate, onSync, onAutoSync, onRemove }: Props) {
  const [remove, setRemove] = useState<RemoveState>({ step: 'idle' })
  const rowBusy = busy === entry.name
  const autoSyncBusy = busy === `${entry.name}/auto-sync`
  const issues = entry.spec.issues ?? []

  const installedIn = (agent: Agent) =>
    entry.links.some((l) => l.agent === agent.id && (project ? l.projectRoot === project : l.scope === 'global'))

  const doRemove = async (force: boolean) => {
    try {
      await onRemove(entry, force)
      setRemove({ step: 'idle' })
    } catch (err) {
      if (err instanceof ApiError && err.status === 409 && !force) {
        setRemove({ step: 'linked', message: err.message })
      } else {
        setRemove({ step: 'idle' })
        throw err
      }
    }
  }

  return (
    <tr className={`row${selected ? ' is-selected' : ''}`}>
      <td className="cell-check">
        <Checkbox checked={selected} label={`Select ${entry.name}`} onChange={(next) => onSelect(entry, next)} />
      </td>
      <td className="cell-name">
        <span className="skill-name">{entry.name}</span>
        <span className="skill-path" title={entry.path}>
          {entry.path}
        </span>
      </td>
      <td className="cell-source">
        <Badge>{entry.source.type}</Badge>{' '}
        {entry.source.url ? (
          <a className="mono" href={entry.source.url} target="_blank" rel="noreferrer" title={entry.source.url}>
            {entry.source.ref}
          </a>
        ) : (
          <span className="mono" title={entry.source.ref}>
            {entry.source.ref}
          </span>
        )}
      </td>
      <td>
        <div className="cell-badges">
          {entry.spec.valid ? (
            <Badge title="Valid against the Agent Skills spec">spec ok</Badge>
          ) : (
            <Badge tone="danger" title={issues.join('\n') || 'Invalid skill'}>
              {`${issues.length || 'spec'} issue${issues.length === 1 ? '' : 's'}`}
            </Badge>
          )}
          {entry.convertedFrom && (
            <Badge tone="info" title={`Converted from ${entry.convertedFrom}`}>
              converted
            </Badge>
          )}
        </div>
      </td>
      {agents.map((a) => {
        const on = installedIn(a)
        const cellBusy = busy === `${entry.name}/${a.id}`
        return (
          <td key={a.id} className="cell-check">
            <Checkbox
              checked={on}
              busy={cellBusy}
              disabled={a.readOnly || rowBusy}
              label={a.readOnly ? `${a.name} is read-only` : `${on ? 'Uninstall from' : 'Install into'} ${a.name}`}
              onChange={(next) => void onToggle(entry, a, next)}
            />
          </td>
        )
      })}
      <td className="cell-check">
        <Toggle
          on={entry.autoSync}
          busy={autoSyncBusy}
          disabled={rowBusy}
          label={`Auto-sync ${entry.name}: ${entry.autoSync ? 'on' : 'off'}`}
          onChange={(next) => void onAutoSync(entry, next)}
        />
      </td>
      <td className="cell-num" title={entry.updatedAt}>
        {shortDate(entry.updatedAt)}
      </td>
      <td className="cell-actions">
        {remove.step === 'idle' ? (
          <>
            <Button variant="ghost" onClick={() => void onUpdate(entry)} disabled={rowBusy} aria-busy={rowBusy}>
              {rowBusy ? <Spinner /> : null}
              Update
            </Button>
            <Button variant="ghost" onClick={() => void onSync(entry)} disabled={rowBusy} title="Link this entry into every enabled agent">
              Sync
            </Button>
            <Button variant="ghost" onClick={() => setRemove({ step: 'confirm' })} disabled={rowBusy}>
              Remove
            </Button>
          </>
        ) : remove.step === 'confirm' ? (
          <InlineConfirm
            message={`Remove ${entry.name} from the vault?`}
            confirmLabel="Remove"
            busy={rowBusy}
            onConfirm={() => void doRemove(false)}
            onCancel={() => setRemove({ step: 'idle' })}
          />
        ) : (
          <InlineConfirm
            message={<span title={remove.message}>Still linked into agents. Unlink and remove?</span>}
            confirmLabel="Remove anyway"
            busy={rowBusy}
            onConfirm={() => void doRemove(true)}
            onCancel={() => setRemove({ step: 'idle' })}
          />
        )}
      </td>
    </tr>
  )
}

function shortDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '–'
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
