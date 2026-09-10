import type { Agent } from '../../api/client'
import { AgentItem } from '../molecules/AgentItem'

export const ALL_AGENTS = 'all'

type Props = {
  agents: Agent[]
  counts: Record<string, number>
  totalCount: number
  selected: string
  busyAgent: string | null
  version: string
  onSelect: (id: string) => void
  onToggle: (id: string, enabled: boolean) => void
}

export function Sidebar({ agents, counts, totalCount, selected, busyAgent, version, onSelect, onToggle }: Props) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand-name">skillman</span>
        <span className="brand-version">{version}</span>
      </div>
      <div className="sidebar-label">Agents</div>
      <ul className="agent-list">
        <li
          className={`agent-item${selected === ALL_AGENTS ? ' is-selected' : ''}`}
          onClick={() => onSelect(ALL_AGENTS)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault()
              onSelect(ALL_AGENTS)
            }
          }}
          tabIndex={0}
          role="button"
          aria-pressed={selected === ALL_AGENTS}
        >
          <span className="agent-name">All</span>
          <span className="agent-count">{totalCount}</span>
          <span style={{ width: 30 }} aria-hidden="true" />
        </li>
        {agents.map((a) => (
          <AgentItem
            key={a.id}
            agent={a}
            count={counts[a.id] ?? 0}
            selected={selected === a.id}
            busy={busyAgent === a.id}
            onSelect={() => onSelect(a.id)}
            onToggle={(enabled) => onToggle(a.id, enabled)}
          />
        ))}
      </ul>
      <div className="sidebar-foot">Disabling an agent hides it from scans. Files are not touched.</div>
    </aside>
  )
}
