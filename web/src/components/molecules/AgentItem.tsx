import type { Agent } from '../../api/client'

type Props = {
  agent: Agent
  count: number
  selected: boolean
  onSelect: () => void
}

/** Sidebar row for one agent. Navigation only; enable/disable lives in Settings. */
export function AgentItem({ agent, count, selected, onSelect }: Props) {
  const cls = ['agent-item', selected ? 'is-selected' : '', agent.enabled ? '' : 'is-off'].join(' ').trim()
  return (
    <li
      className={cls}
      onClick={onSelect}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onSelect()
        }
      }}
      tabIndex={0}
      role="button"
      aria-pressed={selected}
    >
      <span className="agent-name" title={agent.globalDirs[0]}>
        <span className={`agent-dot${agent.enabled && agent.exists ? ' is-on' : ''}`} aria-hidden="true" />
        {agent.name}
      </span>
      <span className="agent-count">{count}</span>
      <span style={{ width: 30 }} aria-hidden="true" />
    </li>
  )
}
