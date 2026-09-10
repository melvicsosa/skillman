import type { Agent, VaultEntry } from '../../api/client'
import { VaultRow } from '../molecules/VaultRow'

type Props = {
  entries: VaultEntry[]
  agents: Agent[]
  project: string | null
  busy: string | null
  onToggle: (entry: VaultEntry, agent: Agent, installed: boolean) => Promise<void>
  onUpdate: (entry: VaultEntry) => Promise<void>
  onRemove: (entry: VaultEntry, force: boolean) => Promise<void>
}

export function VaultTable({ entries, agents, project, busy, onToggle, onUpdate, onRemove }: Props) {
  if (entries.length === 0) {
    return (
      <div className="table-wrap">
        <div className="state-box">
          <h3>The vault is empty</h3>
          <p>Add a skill by ref above, import your lock file, or install one from Discover.</p>
        </div>
      </div>
    )
  }
  return (
    <div className="table-wrap">
      <table className="table table-vault">
        <colgroup>
          <col className="col-name" />
          <col />
          <col className="col-flags" />
          {agents.map((a) => (
            <col key={a.id} className="col-check" />
          ))}
          <col className="col-date" />
          <col className="col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>Name</th>
            <th>Source</th>
            <th>Spec</th>
            {agents.map((a) => (
              <th key={a.id} className="cell-check" title={a.globalDirs[0]}>
                {a.name}
              </th>
            ))}
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
              onToggle={onToggle}
              onUpdate={onUpdate}
              onRemove={onRemove}
            />
          ))}
        </tbody>
      </table>
    </div>
  )
}
