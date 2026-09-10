import type { RegistryResult } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'

type Props = {
  hit: RegistryResult
  inVault: boolean
  onInstall: (hit: RegistryResult) => void
}

export function RegistryRow({ hit, inVault, onInstall }: Props) {
  return (
    <tr className="row">
      <td className="cell-name">
        <span className="skill-name">{hit.name}</span>
        <span className="skill-path mono" title={hit.ref}>
          {hit.ref}
        </span>
        {hit.description && (
          <span className="skill-desc" title={hit.description}>
            {hit.description}
          </span>
        )}
      </td>
      <td className="cell-source">
        {hit.url ? (
          <a href={hit.url} target="_blank" rel="noreferrer" title={hit.url}>
            {hit.source}
          </a>
        ) : (
          hit.source
        )}
      </td>
      <td>
        <Badge>{hit.registry}</Badge>
      </td>
      <td className="cell-num">{hit.installs != null ? formatCount(hit.installs) : '–'}</td>
      <td className="cell-actions">
        {inVault && <Badge tone="info">in vault</Badge>}
        <Button onClick={() => onInstall(hit)}>{inVault ? 'Reinstall' : 'Install'}</Button>
      </td>
    </tr>
  )
}

function formatCount(n: number): string {
  return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(n)
}
