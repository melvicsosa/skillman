import type { Agent } from '../../api/client'
import { AgentItem } from '../molecules/AgentItem'

export const ALL_AGENTS = 'all'
export const VIEW_VAULT = 'vault'
export const VIEW_DISCOVER = 'discover'
export const VIEW_SETTINGS = 'settings'

type Props = {
  agents: Agent[]
  counts: Record<string, number>
  totalCount: number
  vaultCount: number
  selected: string
  busyAgent: string | null
  version: string
  onSelect: (id: string) => void
  onToggle: (id: string, enabled: boolean) => void
}

export function Sidebar({ agents, counts, totalCount, vaultCount, selected, busyAgent, version, onSelect, onToggle }: Props) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <img className="brand-icon" src="/favicon.png" width={22} height={22} alt="" aria-hidden="true" />
        <span className="brand-name">skillman</span>
        <span className="brand-version">{version}</span>
      </div>
      <div className="sidebar-label">Agents</div>
      <ul className="agent-list">
        <NavItem id={ALL_AGENTS} label="All" count={totalCount} selected={selected === ALL_AGENTS} onSelect={onSelect} />
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
      <div className="sidebar-label">Library</div>
      <ul className="agent-list">
        <NavItem id={VIEW_VAULT} label="Vault" count={vaultCount} selected={selected === VIEW_VAULT} onSelect={onSelect} />
        <NavItem id={VIEW_DISCOVER} label="Discover" selected={selected === VIEW_DISCOVER} onSelect={onSelect} />
        <NavItem id={VIEW_SETTINGS} label="Settings" selected={selected === VIEW_SETTINGS} onSelect={onSelect} />
      </ul>
      <div className="sidebar-foot">Disabling an agent hides it from scans. Files are not touched.</div>
    </aside>
  )
}

type NavProps = {
  id: string
  label: string
  count?: number
  selected: boolean
  onSelect: (id: string) => void
}

function NavItem({ id, label, count, selected, onSelect }: NavProps) {
  return (
    <li
      className={`agent-item${selected ? ' is-selected' : ''}`}
      onClick={() => onSelect(id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onSelect(id)
        }
      }}
      tabIndex={0}
      role="button"
      aria-pressed={selected}
    >
      <span className="agent-name">{label}</span>
      <span className="agent-count">{count ?? ''}</span>
      <span style={{ width: 30 }} aria-hidden="true" />
    </li>
  )
}
