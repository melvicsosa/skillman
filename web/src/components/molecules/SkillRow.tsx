import type { Skill } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Toggle } from '../atoms/Toggle'

export type RowFlags = {
  drift?: string
  invalid?: string
}

type Props = {
  skill: Skill
  agentName: string
  showAgent: boolean
  flags: RowFlags
  busy: boolean
  onToggle: (enabled: boolean) => void
}

export function SkillRow({ skill, agentName, showAgent, flags, busy, onToggle }: Props) {
  const disabled = skill.state === 'disabled'
  const scope = skill.projectRoot ? basename(skill.projectRoot) : 'global'
  const shownPath = disabled && skill.quarantinePath ? skill.quarantinePath : skill.path
  return (
    <tr className={`row${disabled ? ' is-disabled' : ''}`}>
      <td className="cell-name">
        <span className="skill-name">{skill.name}</span>
        <span className="skill-path" title={shownPath}>
          {shownPath}
        </span>
      </td>
      <td className="cell-desc" title={skill.description}>
        <div className="desc-clamp">
          {skill.description || <span style={{ color: 'var(--ink-faint)' }}>No description</span>}
        </div>
      </td>
      <td className="cell-num">{skill.version || '–'}</td>
      {showAgent && <td>{agentName}</td>}
      <td title={skill.projectRoot ?? 'Global skill directory'}>{scope}</td>
      <td>
        <div className="cell-badges">
          {skill.isSymlink && <Badge title={`Symlink to ${skill.linkTarget ?? '?'}`}>symlink</Badge>}
          {skill.readOnly && <Badge tone="info" title="Managed by the agent; cannot be disabled here">read-only</Badge>}
          {flags.drift && <Badge tone="warn" title={flags.drift}>drift</Badge>}
          {flags.invalid && <Badge tone="danger" title={flags.invalid}>invalid</Badge>}
        </div>
      </td>
      <td className="cell-state">
        <Toggle
          on={!disabled}
          busy={busy}
          disabled={skill.readOnly}
          onChange={onToggle}
          label={skill.readOnly ? 'Read-only skill' : disabled ? `Enable ${skill.name}` : `Disable ${skill.name}`}
        />
      </td>
    </tr>
  )
}

function basename(p: string): string {
  const parts = p.split('/').filter(Boolean)
  return parts[parts.length - 1] ?? p
}
