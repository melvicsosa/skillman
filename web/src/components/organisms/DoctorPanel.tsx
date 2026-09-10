import type { DoctorReport, Issue } from '../../api/client'
import { Badge } from '../atoms/Badge'

type Props = {
  report: DoctorReport | null
  error: string | null
  loading: boolean
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

export function DoctorPanel({ report, error, loading }: Props) {
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
                    <IssueItem key={`${k}-${idx}`} issue={i} />
                  ))}
              </ul>
            </div>
          ))}
      </div>
    </details>
  )
}

function IssueItem({ issue }: { issue: Issue }) {
  const where = issue.path ?? issue.name ?? ''
  const scope = issue.scope && issue.scope !== 'global' ? issue.scope.replace(/^project:/, '') : issue.scope
  return (
    <li>
      <div className="issue-head">
        <code>{where}</code>
        {issue.agent && <Badge>{issue.agent}</Badge>}
        {scope && issue.kind === 'drift' && <Badge>{scope}</Badge>}
      </div>
      <div className="issue-msg">{issue.message}</div>
      {issue.kind === 'drift' && issue.details && (
        <ul className="issue-details">
          {issue.details.map((d, i) => (
            <li key={i}>{d}</li>
          ))}
        </ul>
      )}
    </li>
  )
}
