import type { RegistryResult } from '../../api/client'
import { RegistryRow } from '../molecules/RegistryRow'
import { EmptyState } from '../molecules/EmptyState'

type Props = {
  hits: RegistryResult[]
  vaultNames: Set<string>
  onInstall: (hit: RegistryResult) => void
  emptyTitle: string
  emptyHint?: string
}

export function RegistryTable({ hits, vaultNames, onInstall, emptyTitle, emptyHint }: Props) {
  if (hits.length === 0) {
    return (
      <div className="table-wrap">
        <EmptyState title={emptyTitle} hint={emptyHint} />
      </div>
    )
  }
  return (
    <div className="table-wrap">
      <table className="table table-registry">
        <colgroup>
          <col className="col-name" />
          <col />
          <col className="col-agent" />
          <col className="col-installs" />
          <col className="col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>Name</th>
            <th>Source</th>
            <th>Registry</th>
            <th>Installs</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {hits.map((h) => (
            <RegistryRow key={`${h.registry}:${h.id || h.ref}`} hit={h} inVault={vaultNames.has(h.name)} onInstall={onInstall} />
          ))}
        </tbody>
      </table>
    </div>
  )
}
