import { useState } from 'react'

import type { DoctorReport, DriftCopy, Issue } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'
import { Disclosure } from '../molecules/Disclosure'
import { EmptyState } from '../molecules/EmptyState'
import { Select } from '../molecules/Select'
import { Spinner } from '../atoms/Spinner'
import type { DoctorView, StatusFilter, UseDoctorFilters } from '../../hooks/useDoctorFilters'

type Props = {
  report: DoctorReport | null
  error: string | null
  loading: boolean
  filters: UseDoctorFilters
  view: DoctorView
  /** Overwrite every other copy of a drifting skill from `from` ('vault' or an agent id). */
  onRepair: (issue: Issue, from: string) => Promise<void>
  /** Copy one agent's version of a drifting skill into the vault. */
  onAdopt: (issue: Issue, from: string) => Promise<void>
}

/** Every kind internal/app/doctor.go can emit; unknown kinds fall back to the raw id. */
const kindLabel: Record<string, string> = {
  'invalid-frontmatter': 'Invalid frontmatter',
  'dangling-symlink': 'Dangling symlinks',
  drift: 'Drift between copies',
  'quarantine-conflict': 'Quarantine conflicts',
  'scan-error': 'Scan errors',
  'vault-broken-link': 'Broken vault links',
  'vault-missing': 'Missing vault entries',
  'rejected-conversion': 'Rejected conversions',
}

const kindTone: Record<string, 'warn' | 'danger' | 'info' | 'neutral'> = {
  'invalid-frontmatter': 'danger',
  'dangling-symlink': 'danger',
  drift: 'warn',
  'quarantine-conflict': 'warn',
  'scan-error': 'danger',
  'vault-broken-link': 'danger',
  'vault-missing': 'warn',
  'rejected-conversion': 'info',
}

const statusLabel: Record<StatusFilter, string> = {
  all: 'All',
  actionable: 'Actionable',
  informational: 'Informational',
}
const statusOrder: StatusFilter[] = ['all', 'actionable', 'informational']

/** Doctor report body grouped by kind; each issue is a collapsed row, at most one open at a time. */
export function DoctorIssues({ report, error, loading, filters, view, onRepair, onAdopt }: Props) {
  const [openId, setOpenId] = useState<string | null>(null)
  const total = report?.issues.length ?? 0
  const kinds = report ? Object.keys(report.counts).sort() : []
  const mixedKinds = kinds.length > 1
  const hidden = new Set(filters.filters.hiddenKinds)

  if (error) {
    return (
      <div className="state-box is-error">
        <h3>Could not run doctor</h3>
        <p>{error}</p>
      </div>
    )
  }
  if (loading && !report) return <div className="state-box">Checking…</div>
  if (!report) return null
  if (total === 0) {
    return <EmptyState title="No issues found" hint="Every skill parses, every symlink resolves, and every copy of each skill matches." />
  }

  const toggle = (id: string) => setOpenId((cur) => (cur === id ? null : id))
  const anyHidden = hidden.size > 0

  return (
    <div className="doctor-issues">
      <div className="doctor-filters">
        <div className="tabs" role="group" aria-label="Issue status">
          {statusOrder.map((s) => (
            <button
              key={s}
              type="button"
              className={`tab${filters.filters.status === s ? ' is-active' : ''}`}
              aria-pressed={filters.filters.status === s}
              onClick={() => filters.setStatus(s)}
            >
              {statusLabel[s]} <span className="tab-count">({view.statusCounts[s]})</span>
            </button>
          ))}
        </div>
        <div className="doctor-kinds" role="group" aria-label="Issue kinds">
          {kinds.map((k) => {
            const on = !hidden.has(k)
            return (
              <button
                key={k}
                type="button"
                className={`badge kind-chip${toneClass(kindTone[k] ?? 'neutral')}${on ? '' : ' is-off'}`}
                aria-pressed={on}
                onClick={() => filters.toggleKind(k)}
                title={on ? 'Hide this kind' : 'Show this kind'}
              >
                {`${kindLabel[k] ?? k} ${view.kindCounts[k] ?? 0}`}
              </button>
            )
          })}
          {anyHidden && (
            <button type="button" className="link doctor-kinds-reset" onClick={filters.resetKinds}>
              Reset
            </button>
          )}
        </div>
      </div>
      {view.visible.length === 0 ? (
        <div className="doctor-filters-empty">
          <EmptyState title="No issues match these filters" hint="Switch the status filter or re-select a kind to see them again." />
          <Button variant="ghost" onClick={filters.showAll}>
            Reset filters
          </Button>
        </div>
      ) : (
        kinds.map((k) => {
          const rows = view.visible.filter(({ issue }) => issue.kind === k)
          if (rows.length === 0) return null
          return (
            <div className="doctor-group" key={k}>
              <h4>
                {kindLabel[k] ?? k} ({rows.length})
              </h4>
              <ul>
                {rows.map(({ issue, id }) => (
                  <IssueItem
                    key={id}
                    issue={issue}
                    open={openId === id}
                    onToggle={() => toggle(id)}
                    showKind={mixedKinds}
                    onRepair={onRepair}
                    onAdopt={onAdopt}
                  />
                ))}
              </ul>
            </div>
          )
        })
      )}
    </div>
  )
}

function toneClass(tone: 'warn' | 'danger' | 'info' | 'neutral'): string {
  return tone === 'neutral' ? '' : ` is-${tone}`
}

type ItemProps = {
  issue: Issue
  open: boolean
  onToggle: () => void
  showKind: boolean
  onRepair: (issue: Issue, from: string) => Promise<void>
  onAdopt: (issue: Issue, from: string) => Promise<void>
}

/** One-line muted summary for the collapsed row. */
function issueSummary(issue: Issue, copies: DriftCopy[], allReadOnly: boolean): string {
  if (issue.kind === 'drift' && copies.length > 0) {
    const hashes = new Set(copies.map((c) => c.hash)).size
    const parts = [`${copies.length} ${copies.length === 1 ? 'copy' : 'copies'}`, `${hashes} ${hashes === 1 ? 'hash' : 'hashes'}`]
    if (allReadOnly) parts.push('read-only')
    return parts.join(' · ')
  }
  return issue.message
}

function IssueItem({ issue, open, onToggle, showKind, onRepair, onAdopt }: ItemProps) {
  const name = issue.name ?? issue.path ?? ''
  const scope = issue.scope && issue.scope !== 'global' ? issue.scope.replace(/^project:/, '') : issue.scope
  const isDrift = issue.kind === 'drift'
  const copies = issue.copies ?? []
  const allReadOnly = isDrift && copies.length > 0 && copies.every((c) => c.readOnly)
  const summary = issueSummary(issue, copies, allReadOnly)
  const tone = allReadOnly ? 'neutral' : (kindTone[issue.kind] ?? 'neutral')

  const header = (
    <>
      <span className="issue-name">{name}</span>
      {scope && <Badge>{scope}</Badge>}
      {showKind && <Badge tone={kindTone[issue.kind] ?? 'neutral'}>{kindLabel[issue.kind] ?? issue.kind}</Badge>}
      <span className="issue-summary" title={summary}>
        {summary}
      </span>
    </>
  )

  return (
    <li className={`issue-item tone-${tone}`}>
      <Disclosure summary={header} open={open} onToggle={onToggle} className="issue-disclosure">
        <div className="issue-head">
          {issue.agent && <Badge>{issue.agent}</Badge>}
          {isDrift && issue.vaultRef && <Badge tone="info">{`vault: ${issue.vaultRef}`}</Badge>}
          {issue.path && issue.path !== name && (
            <code className="issue-path" title={issue.path}>
              {issue.path}
            </code>
          )}
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
        {allReadOnly ? (
          <div className="drift-readonly">All copies are read-only; nothing to do.</div>
        ) : (
          isDrift && copies.length > 0 && <DriftActions issue={issue} onRepair={onRepair} onAdopt={onAdopt} />
        )}
      </Disclosure>
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

type ActionProps = {
  issue: Issue
  onRepair: (issue: Issue, from: string) => Promise<void>
  onAdopt: (issue: Issue, from: string) => Promise<void>
}

function DriftActions({ issue, onRepair, onAdopt }: ActionProps) {
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
      <div className="drift-action">
        <span>Repair from</span>
        <Select
          ariaLabel="Repair from"
          value={from}
          onChange={setFrom}
          disabled={busy !== null || !canRepair}
          options={sources.map((s) => ({ value: s, label: s }))}
        />
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
      </div>
      {!issue.vaultRef && agents.length > 0 && (
        <div className="drift-action">
          <span>Adopt into vault from</span>
          <Select
            ariaLabel="Adopt into vault from"
            value={adoptFrom}
            onChange={setAdoptFrom}
            disabled={busy !== null}
            options={agents.map((a) => ({ value: a, label: a }))}
          />
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
        </div>
      )}
    </div>
  )
}
