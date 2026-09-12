import type { Agent } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'
import { Spinner } from '../atoms/Spinner'
import { Toggle } from '../atoms/Toggle'

type Props = {
  agents: Agent[]
  /** Agent id whose enable/disable request is in flight. */
  busyAgent: string | null
  redetecting: boolean
  onToggle: (id: string, enabled: boolean) => void
  onRedetect: () => void
}

/** Every known agent (enabled or not) with its on/off switch and a re-detect action. */
export function AgentsCard({ agents, busyAgent, redetecting, onToggle, onRedetect }: Props) {
  return (
    <section className="card">
      <header className="card-head">
        <h2>Agents</h2>
      </header>
      <p className="card-hint">Disabled agents are hidden from the sidebar and skipped by scans. Files are never touched.</p>
      <ul className="agent-rows">
        {agents.map((a) => (
          <li key={a.id} className={`agent-row${a.enabled ? '' : ' is-off'}`}>
            <span className={`agent-dot${a.enabled && a.exists ? ' is-on' : ''}`} aria-hidden="true" />
            <span className="agent-row-text">
              <span className="agent-row-name">
                {a.name}
                {!a.exists && <Badge>not detected</Badge>}
                {a.readOnly && <Badge>read-only</Badge>}
              </span>
              {a.globalDirs[0] && (
                <span className="agent-row-dir mono" title={a.globalDirs.join('\n')}>
                  {a.globalDirs[0]}
                </span>
              )}
            </span>
            <span className="agent-count">{a.skillCount}</span>
            <Toggle
              on={a.enabled}
              busy={busyAgent === a.id}
              onChange={(enabled) => onToggle(a.id, enabled)}
              label={a.enabled ? `Disable ${a.name}` : `Enable ${a.name}`}
            />
          </li>
        ))}
      </ul>
      <div className="card-actions">
        <Button onClick={onRedetect} disabled={redetecting} aria-busy={redetecting}>
          {redetecting ? <Spinner /> : null}
          Re-detect agents
        </Button>
        <span className="hint">Run this after installing a new AI agent.</span>
      </div>
    </section>
  )
}
