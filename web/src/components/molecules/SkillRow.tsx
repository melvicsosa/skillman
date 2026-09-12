import { useLayoutEffect, useRef, useState, type RefObject } from 'react'
import type { Skill } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Toggle } from '../atoms/Toggle'

export type RowFlags = {
  drift?: string
  invalid?: string
  /** Vault entry name when the skill is linked from the vault. */
  vault?: string
}

type Props = {
  skill: Skill
  agentName: string
  showAgent: boolean
  showUsage: boolean
  flags: RowFlags
  busy: boolean
  /** Show the full description instead of the two-line clamp. */
  expanded: boolean
  onToggleExpanded: () => void
  onToggle: (enabled: boolean) => void
}

export function SkillRow({ skill, agentName, showAgent, showUsage, flags, busy, expanded, onToggleExpanded, onToggle }: Props) {
  const disabled = skill.state === 'disabled'
  const scope = skill.projectRoot ? basename(skill.projectRoot) : 'global'
  const shownPath = disabled && skill.quarantinePath ? skill.quarantinePath : skill.path
  const [descRef, truncated] = useTruncated(skill.description, expanded)
  return (
    <tr className={`row${disabled ? ' is-disabled' : ''}`}>
      <td className="cell-name">
        <span className="skill-name">{skill.name}</span>
        <span className="skill-path" title={shownPath}>
          {shownPath}
        </span>
      </td>
      <td className="cell-desc" title={expanded ? undefined : skill.description}>
        <div ref={descRef} className={expanded ? 'desc-full' : 'desc-clamp'}>
          {skill.description || <span style={{ color: 'var(--ink-faint)' }}>No description</span>}
        </div>
        {(expanded || truncated) && (
          <button
            type="button"
            className="link desc-more"
            aria-expanded={expanded}
            onClick={(e) => {
              e.stopPropagation()
              onToggleExpanded()
            }}
          >
            {expanded ? 'less' : 'more'}
          </button>
        )}
      </td>
      <td className="cell-num">{skill.version || '–'}</td>
      {showAgent && <td>{agentName}</td>}
      <td title={skill.projectRoot ?? 'Global skill directory'}>{scope}</td>
      {showUsage && (
        <td className="cell-used" title={skill.lastUsedAt ? `Last used ${new Date(skill.lastUsedAt).toLocaleString()}` : undefined}>
          {skill.usageCount > 0 ? <span className="used-badge">Used {skill.usageCount}×</span> : null}
        </td>
      )}
      <td>
        <div className="cell-badges">
          {skill.isSymlink && <Badge title={`Symlink to ${skill.linkTarget ?? '?'}`}>symlink</Badge>}
          {skill.readOnly && <Badge tone="info" title="Managed by the agent; cannot be disabled here">read-only</Badge>}
          {flags.vault && <Badge tone="info" title={`Installed from vault entry ${flags.vault}`}>vault</Badge>}
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

/** Whether the clamped description overflows its box; re-measured on resize and text change. */
function useTruncated(text: string, expanded: boolean): [RefObject<HTMLDivElement | null>, boolean] {
  const ref = useRef<HTMLDivElement>(null)
  const [value, setValue] = useState(false)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el || expanded) return
    const measure = () => setValue(el.scrollHeight > el.clientHeight + 1)
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [text, expanded])
  return [ref, value]
}

function basename(p: string): string {
  const parts = p.split('/').filter(Boolean)
  return parts[parts.length - 1] ?? p
}
