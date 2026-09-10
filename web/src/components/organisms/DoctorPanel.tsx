import { useState } from 'react'

import type { DoctorReport, DriftCopy, Issue } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'
import { Spinner } from '../atoms/Spinner'

type Props = {
  report: DoctorReport | null
  error: string | null
  loading: boolean
  /** Overwrite every other copy of a drifting skill from `from` ('vault' or an agent id). */
  onRepair: (issue: Issue, from: string) => Promise<void>
  /** Copy one agent's version of a drifting skill into the vault. */
  onAdopt: (issue: Issue, from: string) => Promise<void>
}

const kindLabel: Record<string, string> = {
  'invalid-frontmatter': 'Invalid frontmatter',
  'dangling-symlink': 'Dangling symlinks',
  drift: 'Drift between copies',
  'quarantine-conflict': 'Quarantine conflicts',
  'scan-error': 'Scan errors',
}

const kindTone: Record<string, 'warn' | 'danger' | 'info' | 'neutral'> = {
  'invalid-frontmatter': 'danger',
  'dangling-symlink': 'danger',
  drift: 'warn',
  'quarantine-conflict': 'warn',
  'scan-error': 'danger',
}

export function DoctorPanel({ report, error, loading, onRepair, onAdopt }: Props) {
  const total = report?.issues.length ?? 0
  const kinds = report ? Object.keys(report.counts).sort() : []
  return (
    <details className="doctor">
      <summary>
        <span className="doctor-title">Doctor</span>
        {loading && !report && <span style={{ color: 'var(--ink-faint)' }}>checking…</span>}
        {error && <Badge tone="danger">{error}</Badge>}
        {report && total === 0 && <Badge>no issues</Badge>}
        {report && total > 0 && (
          <span className="doctor-counts">
            {kinds.map((k) => (
              <Badge key={k} tone={kindTone[k] ?? 'neutral'}>{`${kindLabel[k] ?? k}: ${report.counts[k]}`}</Badge>
            ))}
          </span>
        )}
      </summary>
      <div className="doctor-body">
        {report && total === 0 && <p style={{ color: 'var(--ink-muted)' }}>Every skill parses, every symlink resolves, and every copy of each skill matches.</p>}
        {report &&
          kinds.map((k) => (
            <div className="doctor-group" key={k}>
              <h4>
                {kindLabel[k] ?? k} ({report.counts[k]})
              </h4>
              <ul>
                {report.issues
                  .filter((i) => i.kind === k)
                  .map((i, idx) => (
                    <IssueItem key={`${k}-${idx}`} issue={i} onRepair={onRepair} onAdopt={onAdopt} />
                  ))}
              </ul>
            </div>
          ))}
      </div>
    </details>
  )
}

type ItemProps = {
  issue: Issue
  onRepair: (issue: Issue, from: string) => Promise<void>
  onAdopt: (issue: Issue, from: string) => Promise<void>
}

function IssueItem({ issue, onRepair, onAdopt }: ItemProps) {
  const where = issue.path ?? issue.name ?? ''
  const scope = issue.scope && issue.scope !== 'global' ? issue.scope.replace(/^project:/, '') : issue.scope
  const isDrift = issue.kind === 'drift'
  const copies = issue.copies ?? []
  return (
    <li>
      <div className="issue-head">
        <code>{where}</code>
        {issue.agent && <Badge>{issue.agent}</Badge>}
        {scope && isDrift && <Badge>{scope}</Badge>}
        {isDrift && issue.vaultRef && <Badge tone="info">{`vault: ${issue.vaultRef}`}</Badge>}
      </div>
      <div className="issue-msg">{issue.message}</div>
      {isDrift && copies.length > 0 ? (
        <ul className="issue-copies">
          {copies.map((c) => (
            <CopyLine key={c.skillId || c.path} copy={c} />
          ))}
        </ul>
      ) : (
        isDrift &&
        issue.details && (
          <ul className="issue-details">
            {issue.details.map((d, i) => (
              <li key={i}>{d}</li>
            ))}
          </ul>
        )
      )}
      {isDrift && copies.length > 0 && <DriftActions issue={issue} onRepair={onRepair} onAdopt={onAdopt} />}
    </li>
  )
}

function CopyLine({ copy }: { copy: DriftCopy }) {
  const notes: string[] = []
  if (copy.isSymlink) notes.push(copy.vaultRef ? `symlink -> vault (${copy.vaultRef})` : 'symlink')
  if (copy.readOnly) notes.push('read-only')
  if (!copy.repairable) notes.push(`not repairable${copy.reason ? `: ${copy.reason}` : ''}`)
  return (
    <li className="issue-copy">
      <Badge>{copy.agent}</Badge>
      <span className="mono issue-copy-hash" title={copy.hash}>
        {copy.hash.slice(0, 8)}
      </span>
      <span className="mono issue-copy-path" title={copy.path}>
        {copy.path}
      </span>
      {notes.length > 0 && <span className="hint">{notes.join(' · ')}</span>}
    </li>
  )
}

function DriftActions({ issue, onRepair, onAdopt }: ItemProps) {
  const copies = issue.copies ?? []
  const agents = [...new Set(copies.map((c) => c.agent))]
  const sources = issue.vaultRef ? ['vault', ...agents] : agents
  const [from, setFrom] = useState(sources[0] ?? '')
  const [adoptFrom, setAdoptFrom] = useState(agents[0] ?? '')
  const [busy, setBusy] = useState<'repair' | 'adopt' | null>(null)

  const run = async (kind: 'repair' | 'adopt', fn: () => Promise<void>) => {
    setBusy(kind)
    try {
      await fn()
    } finally {
      setBusy(null)
    }
  }

  const canRepair = issue.repairable !== false && sources.length > 0
  return (
    <div className="drift-actions">
      <label className="drift-action">
        <span>Repair from</span>
        <select className="select" value={from} onChange={(e) => setFrom(e.target.value)} disabled={busy !== null || !canRepair}>
          {sources.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
        <Button
          variant="ghost"
          onClick={() => void run('repair', () => onRepair(issue, from))}
          disabled={busy !== null || !canRepair || !from}
          aria-busy={busy === 'repair'}
          title={canRepair ? 'Overwrite the other copies with this source' : 'No copy of this skill can be replaced'}
        >
          {busy === 'repair' ? <Spinner /> : null}
          Repair
        </Button>
      </label>
      {!issue.vaultRef && agents.length > 0 && (
        <label className="drift-action">
          <span>Adopt into vault from</span>
          <select className="select" value={adoptFrom} onChange={(e) => setAdoptFrom(e.target.value)} disabled={busy !== null}>
            {agents.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
          <Button
            variant="ghost"
            onClick={() => void run('adopt', () => onAdopt(issue, adoptFrom))}
            disabled={busy !== null || !adoptFrom}
            aria-busy={busy === 'adopt'}
            title="Copy this agent's version into the vault so it becomes the source of truth"
          >
            {busy === 'adopt' ? <Spinner /> : null}
            Adopt
          </Button>
        </label>
      )}
    </div>
  )
}
