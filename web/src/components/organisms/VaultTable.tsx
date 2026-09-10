import type { Agent, VaultEntry } from '../../api/client'
import { Checkbox } from '../atoms/Checkbox'
import { VaultRow } from '../molecules/VaultRow'
import { EmptyState } from '../molecules/EmptyState'

type Props = {
  entries: VaultEntry[]
  agents: Agent[]
  project: string | null
  busy: string | null
  /** Names of the selected entries (export). */
  selected: Set<string>
  onSelect: (entry: VaultEntry, selected: boolean) => void
  /** Select or clear every entry currently listed. */
  onSelectAll: (selected: boolean) => void
  onToggle: (entry: VaultEntry, agent: Agent, installed: boolean) => Promise<void>
  onUpdate: (entry: VaultEntry) => Promise<void>
  onSync: (entry: VaultEntry) => Promise<void>
  onAutoSync: (entry: VaultEntry, autoSync: boolean) => Promise<void>
  onRemove: (entry: VaultEntry, force: boolean) => Promise<void>
}

export function VaultTable({
  entries,
  agents,
  project,
  busy,
  selected,
  onSelect,
  onSelectAll,
  onToggle,
  onUpdate,
  onSync,
  onAutoSync,
  onRemove,
}: Props) {
  if (entries.length === 0) {
    return (
      <div className="table-wrap">
        <EmptyState
          title="The vault is empty"
          hint="Add a skill by ref above, import your lock file, or install one from Discover."
        />
      </div>
    )
  }
  const allSelected = entries.every((e) => selected.has(e.name))
  return (
    <div className="table-wrap">
      <table className="table table-vault">
        <colgroup>
          <col className="col-select" />
          <col className="col-name" />
          <col />
          <col className="col-flags" />
          {agents.map((a) => (
            <col key={a.id} className="col-check" />
          ))}
          <col className="col-check" />
          <col className="col-date" />
          <col className="col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th className="cell-check">
              <Checkbox checked={allSelected} label={allSelected ? 'Clear selection' : 'Select all listed'} onChange={onSelectAll} />
            </th>
            <th>Name</th>
            <th>Source</th>
            <th>Spec</th>
            {agents.map((a) => (
              <th key={a.id} className="cell-check" title={a.globalDirs[0]}>
                {a.name}
              </th>
            ))}
            <th className="cell-check" title="Link into every enabled agent after each scan">
              Auto-sync
            </th>
            <th>Updated</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {entries.map((e) => (
            <VaultRow
              key={e.name}
              entry={e}
              agents={agents}
              project={project}
              busy={busy}
              selected={selected.has(e.name)}
              onSelect={onSelect}
              onToggle={onToggle}
              onUpdate={onUpdate}
              onSync={onSync}
              onAutoSync={onAutoSync}
              onRemove={onRemove}
            />
          ))}
        </tbody>
      </table>
    </div>
  )
}
