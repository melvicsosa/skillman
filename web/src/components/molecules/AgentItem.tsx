import type { Agent } from '../../api/client'
import { Toggle } from '../atoms/Toggle'

type Props = {
  agent: Agent
  count: number
  selected: boolean
  busy: boolean
  onSelect: () => void
  onToggle: (enabled: boolean) => void
}

export function AgentItem({ agent, count, selected, busy, onSelect, onToggle }: Props) {
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
      <Toggle
        on={agent.enabled}
        busy={busy}
        onChange={onToggle}
        label={agent.enabled ? `Disable ${agent.name}` : `Enable ${agent.name}`}
      />
    </li>
  )
}
